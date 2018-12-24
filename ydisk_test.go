package ydisk

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/slytomcat/llog"
)

var (
	Cfg, CfgPath, Dir string
	YD                *YDisk
)

const (
	SyncDir        = "$HOME/TeSt_Yandex.Disk_TeSt"
	ConfigFilePath = "$HOME/.config/TeSt_Yandex.Disk_TeSt"
)

func init() {
	llog.SetLevel(llog.DEBUG)
	llog.SetFlags(log.Lshortfile | log.Lmicroseconds)
	CfgPath = os.ExpandEnv(ConfigFilePath)
	Cfg = filepath.Join(CfgPath, "config.Cfg")
	Dir = os.ExpandEnv(SyncDir)
	os.Setenv("DEBUG_SyncDir", Dir)
	// instll simulator for yandex-disk
	exec.Command("go", "get", "github.com/slytomcat/yandex-disk-simulator").Run()
	// rename simulator to original utility name
	exe, err := exec.LookPath("yandex-disk-simulator")
	if err != nil {
		log.Fatal("yandex-disk simulator installation error:", err)
	}
	Dir, _ := filepath.Split(exe)
	exec.Command("mv", exe, filepath.Join(Dir, "yandex-disk")).Run()
	os.Setenv("PATH", Dir+":"+os.Getenv("PATH"))
	llog.Debug("Init completed")
}

func TestNotInstalled(t *testing.T) {
	path := os.Getenv("PATH")
	// make PATH empty for test time
	os.Setenv("PATH", "")
	// test not_installed case
	_, err := NewYDisk(Cfg)
	if err == nil {
		t.Error("Initialized with not installed daemon")
	}
	// restore original PATH value
	os.Setenv("PATH", path)
}

func TestWrongConf(t *testing.T) {
	// test initialization with wrong/not-existing config
	_, err := NewYDisk(Cfg + "_bad")
	if err == nil {
		t.Error("Initialized with not existing daemon config file")
	}
}

func TestEmptyConf(t *testing.T) {
	// test initialization with empty config
	file, err := os.OpenFile(Cfg, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		llog.Error(err)
	} else {
		file.Write([]byte("Dir=\"no_dir\"\n\nproxy=\"no\"\n"))
		file.Close()
		defer os.Remove(Cfg)

		_, err = NewYDisk(Cfg)
		if err == nil {
			t.Error("Initialized with empty config file")
		}

	}
}

func TestCreateSuccess(t *testing.T) {
	// setup yandex-disk
	err := os.MkdirAll(CfgPath, 0777)
	if err != nil {
		t.Error("config path creation error")
	}
	auth := filepath.Join(CfgPath, "passwd")
	if notExists(auth) {
		file, err := os.OpenFile(auth, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			llog.Critical("yandex-disk token file creation error:", err)
		}
		_, err = file.Write([]byte("token")) // yandex-disk-simulator doesn't require the real token
		if err != nil {
			llog.Critical("yandex-disk token file creation error:", err)
		}
		file.Close()
	}
	file, err := os.OpenFile(Cfg, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		llog.Critical(err)
	} else {
		_, err := file.Write([]byte("proxy=\"no\"\nauth=\"" + auth + "\"\ndir=\"" + Dir + "\"\n\n"))
		if err != nil {
			t.Error("Can't create config file: ", err)
		}
	}
	err = os.MkdirAll(Dir, 0777)
	if err != nil {
		t.Error("synchronization Dir creation error:", err)
	}
	YD, err = NewYDisk(Cfg)
	if err != nil {
		t.Error("creation error of normally configured daemon")
	}
}

func TestOutputNotStarted(t *testing.T) {
	output := YD.Output()
	if output != "" {
		t.Error("Non-empty response from inactive daemon")
	}
}

func TestInitialEvent(t *testing.T) {
	var yds YDvals
	select {
	case yds = <-YD.Changes:
		llog.Infof("%v", yds)
	case <-time.After(time.Second):
		t.Error("No events received within 1 sec interval after YDisk creation")
	}
	if yds.Stat != "none" {
		t.Error("Not 'none' status received from inactive daemon")
	}
}

func TestStart(t *testing.T) {
	var yds YDvals
	YD.Start()
	select {
	case yds = <-YD.Changes:
		llog.Infof("%v", yds)
	case <-time.After(time.Second * 3):
		t.Error("No events received within 3 sec interval after daemon start")
	}
	if yds.Stat == "none" {
		t.Error("'none' status received from started daemon")
	}
}

func TestOutputStarted(t *testing.T) {
	output := YD.Output()
	if output == "" {
		t.Error("Empty response from started daemon")
	}
}

func TestStart2Idle(t *testing.T) {
	var yds YDvals
	for {
		select {
		case yds = <-YD.Changes:
			if yds.Stat == "idle" {
				return
			}
		case <-time.After(time.Second * 30):
			t.Error("No 'idle' status received within 30 sec interval after daemon start")
			return
		}
	}
}

func TestSecondaryStart(t *testing.T) {
	YD.Start()
	select {
	case <-YD.Changes:
		t.Error("Event received within 3 sec interval after secondary start of daemon")
	case <-time.After(time.Second * 3):
	}
}

func TestReaction(t *testing.T) {
	_ = exec.Command("yandex-disk", "sync").Run()
	select {
	case yds := <-YD.Changes:
		if yds.Stat == "index" || yds.Stat == "busy" {
			return
		}
		t.Error("Not index/busy status received after sync started")
	case <-time.After(time.Second * 2):
		t.Error("No reaction within 2 seconds after sync started")
	}
}

func TestBusy2Idle(t *testing.T) {
	var yds YDvals
	for {
		select {
		case yds = <-YD.Changes:
			if yds.Stat == "idle" {
				return
			}
		case <-time.After(time.Second * 10):
			t.Error("No 'idle' status received within 10 sec interval after sync start")
			return
		}
	}
}
func TestStop(t *testing.T) {
	var yds YDvals
	YD.Stop()
	for {
		select {
		case yds = <-YD.Changes:
			if yds.Stat == "none" {
				return
			}
		case <-time.After(time.Second * 3):
			t.Error("'none' status not received within 3 sec interval after daemon stop")
			return
		}
	}
}

func TestSecondaryStop(t *testing.T) {
	YD.Stop()
	select {
	case <-YD.Changes:
		t.Error("Event received within 3 sec interval after secondary stop of daemon")
	case <-time.After(time.Second * 3):
	}
}

func TestClose(t *testing.T) {
	YD.Close()
	select {
	case _, ok := <-YD.Changes:
		if ok {
			t.Error("Event received after YDisk.Close()")
		} else {
			return // Channel closed - it's Ok.
		}
	case <-time.After(time.Second):
		t.Error("Events channel is not closed after YDisk.Close()")
	}
}
