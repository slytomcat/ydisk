package ydisk

import (
	"time"
	"os/exec"
	"os"
	"path/filepath"
	"testing"

	"github.com/slytomcat/llog"
)

var (
	home, cfg, cfgpath string
	YD *YDisk
)

func init() {
	llog.SetLevel(llog.DEBUG)
	home = os.Getenv("HOME")
	cfgpath = filepath.Join(home, ".config", "yandex-disk")
	cfg = filepath.Join(cfgpath, "config.cfg")
	
}

func TestNotInstalled(t *testing.T) {
	// look for installed yandex-disk daemon
	daemon, err  := exec.LookPath("yandex-disk")
	notInstalled := true
	if err == nil {
		llog.Info("yandex-disk installed. Try to rename it for NOT_INSTALLED case test")
		err = exec.Command("sudo", "mv", daemon, daemon+"_").Run()
		if err != nil {
			llog.Error(err," Can't rename yandex-disk: NOT_INSTALLED case can't be tested")
			notInstalled = false
		}
		defer func () {
			_ = exec.Command("sudo", "mv", daemon+"_", daemon).Run()
		}()
	}	

	if notInstalled {
		// test not_installed case
		llog.Info("NOT_INSTALLED case test")
		_, err = NewYDisk(cfg)
		if err == nil {
			t.Error("Initialized with not installed daemon")
		}
	}
}

func TestWrongConf(t *testing.T) {
	// test initialization with wrong config 
	llog.Info("WRONG_CONF case test")
	_, err := NewYDisk(cfg+"_bad")
	if err == nil {
		t.Error("Initialized with not existing daemon config file")
	}
}

func TestEmptyConf(t *testing.T) {
	// test initialization with empty config
	ecfg := "empty.cfg"
	file, err := os.OpenFile(ecfg, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		llog.Error(err)	
	} else {
		file.Write([]byte(`dir="no_dir"
auth="no_auth"
proxy="no"
`))
		file.Close()
		defer os.Remove(ecfg)

		llog.Info("EMPTY_CONF case test")
		_, err = NewYDisk(ecfg)
		if err == nil {
			t.Error("Initialized with empty config file")
		}

	}
}

func TestCreateSuccess(t *testing.T) {
	// setup yandex-disk
	err := exec.Command("yandex-disk", "token", "-p", "$YPASS", "$YUSER").Run()
	if err != nil{
		llog.Error(err)
	}
	file, err := os.OpenFile(cfg, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		llog.Error(err)	
	} else {
		file.Write([]byte(`proxy="no"
dir="`+filepath.Join(cfgpath, "passwd")+`"
auth="`+cfg+`"
`))
	}
	YD, err = NewYDisk(cfg)
	if err != nil {
		t.Error("Unsuccessful start of configuret daemon")
	}
}

func TestOutputNotStarted(t *testing.T) {
	output := YD.Output()
	if output != "" {
		t.Error("Non empty response from unactive daemon")
	}
}

func TestInitialEvent(t *testing.T) {
	var yds YDvals
	select{
	case yds = <-YD.Changes:
		llog.Infof("%v", yds)
	case <- time.After(time.Second):
		t.Error("No Events received within 1 sec interval after YDisk creation")
	}
	if yds.Stat != "none" {
		t.Error("Not 'none' status received from unactive daemon")
	}
}

func TestStart(t *testing.T) {
	var yds YDvals
	YD.Start()
	select{
	case yds = <-YD.Changes:
		llog.Infof("%v", yds)
	case <- time.After(time.Second * 3):
		t.Error("No Events received within 3 sec interval after daemon start")
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
			if yds.Stat == "idle"{
				return
			}
		case <- time.After(time.Second * 20):
			t.Error("No 'idle' status received within 20 sec interval after daemon start")
			return
		}
	}
}

func TestSecondaryStart(t *testing.T) {
	YD.Start()
	select{
	case <-YD.Changes:
		t.Error("Event received within 3 sec interval after secondary start of daemon")
	case <- time.After(time.Second * 3):
	}
}

func TestStop(t *testing.T) {
	var yds YDvals
	YD.Stop()
	for {
		select {
		case yds = <-YD.Changes:
			if yds.Stat == "none"{
				return
			}
		case <- time.After(time.Second * 3):
			t.Error("'none' status not received within 3 sec interval after daemon stop")
			return
		}
	}
}

func TestSecondaryStop(t *testing.T) {
	YD.Stop()
	select{
	case <-YD.Changes:
		t.Error("Event received within 3 sec interval after secondary stop of daemon")
	case <- time.After(time.Second * 3):
	}
}


func TestClose(t *testing.T) {
	YD.Close()
	select{
	case _, ok := <-YD.Changes:
		if ok {
			t.Error("Event received after YDisk.Close()")
		}
	case <- time.After(time.Second):
		t.Error("Events channel is not closed after YDisk.Close()")
	}
}