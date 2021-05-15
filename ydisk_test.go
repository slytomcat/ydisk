package ydisk

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/slytomcat/llog"
)

var (
	Cfg, CfgPath, SyncDir, SymExe string
	YD                            *YDisk
)

const (
	SyncDirPath    = "$HOME/TeSt_Yandex.Disk_TeSt"
	ConfigFilePath = "$HOME/.config/TeSt_Yandex.Disk_TeSt"
)

func TestMain(m *testing.M) {
	flag.Parse()

	// Initialization
	llog.SetLevel(llog.DEBUG)
	llog.SetFlags(log.Lshortfile | log.Lmicroseconds)
	CfgPath = os.ExpandEnv(ConfigFilePath)
	Cfg = filepath.Join(CfgPath, "config.cfg")
	SyncDir = os.ExpandEnv(SyncDirPath)
	os.Setenv("Sim_SyncDir", SyncDir)
	os.Setenv("Sim_ConfDir", CfgPath)
	err := os.MkdirAll(CfgPath, 0755)
	if err != nil {
		log.Fatal(CfgPath, " creation error:", err)
	}
	// get and build simulator for yandex-disk
	// err = exec.Command("go", "get", "-u", "github.com/slytomcat/yandex-disk-simulator").Run()
	// if err != nil {
	// 	log.Fatal("yandex-disk simulator building error:", err)
	// }
	// rename simulator to original utility name
	// exe, err := exec.LookPath("yandex-disk-simulator")
	// if err != nil {
	// 	log.Fatal("yandex-disk simulator installation error:", err)
	// }
	// exeDir, _ := filepath.Split(exe)
	// SymExe = filepath.Join(exeDir, "./yandex-disk")
	// exec.Command("mv", exe, SymExe).Run()
	// os.Setenv("PATH", exeDir+":"+os.Getenv("PATH"))

	SymExe, err = exec.LookPath("yandex-disk")
	if err != nil {
		log.Fatal("yandex-disk utility lookup error:", err)
	}

	exec.Command(SymExe, "stop").Run()
	os.RemoveAll(path.Join(os.TempDir(), "yandexdisksimulator.socket"))
	log.Printf("Tests init completed: yd exe: %v", SymExe)

	// Run tests
	e := m.Run()

	// Clearance
	exec.Command(SymExe, "stop").Run()
	os.RemoveAll(path.Join(os.TempDir(), "yandexdisksimulator.socket"))
	os.RemoveAll(CfgPath)
	os.RemoveAll(SyncDir)
	log.Println("Tests clearance completed")
	os.Exit(e)
}

func TestNotInstalled(t *testing.T) {
	// defer restore original PATH value
	defer func(p string) {
		os.Setenv("PATH", p)
	}(os.Getenv("PATH"))
	// make PATH empty for test time
	os.Setenv("PATH", "")
	// test not_installed case
	_, err := NewYDisk(Cfg)
	if err == nil {
		t.Error("Initialized with not installed daemon")
	}
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
	// prepare for similation
	err := exec.Command(SymExe, "setup").Run()
	if err != nil {
		t.Fatalf("simulation prepare error: %v", err)
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
		if fmt.Sprintf("%v", yds) != "{none unknown     [] true   }" {
			t.Error("Incorrect change object:", yds)
		}
	case <-time.After(time.Second):
		t.Error("No events received within 1 sec interval after YDisk creation")
	}
}

func TestStart(t *testing.T) {
	var yds YDvals
	err := YD.Start()
	if err != nil {
		t.Error("daemon start error:", err)
	}
	select {
	case yds = <-YD.Changes:
		if fmt.Sprintf("%v", yds) != "{paused none     [File.ods downloads/file.deb downloads/setup download down do d o w n] true   }" {
			t.Error("Incorrect change object:", yds)
		}
	case <-time.After(time.Second * 3):
		t.Error("No events received within 3 sec interval after daemon start")
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
				if fmt.Sprintf("%v", yds) != "{idle index 43.50 GB 2.89 GB 40.61 GB 0 B [File.ods downloads/file.deb downloads/setup download down do d o w n] false   }" {
					t.Error("Incorrect change object:", yds)
				}
				return
			}
		case <-time.After(time.Second * 30):
			t.Error("No 'idle' status received within 30 sec interval after daemon start")
			return
		}
	}
}

func TestSecondaryStart(t *testing.T) {
	err := YD.Start()
	if err != nil {
		t.Error("daemon start error:", err)
	}
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
			if fmt.Sprintf("%v", yds) != "{index idle 43.50 GB 2.89 GB 40.61 GB 0 B [File.ods downloads/file.deb downloads/setup download down do d o w n] false   }" {
				t.Error("Incorrect change object:", yds)
			}
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
				if fmt.Sprintf("%v", yds) != "{idle index 43.50 GB 2.89 GB 40.61 GB 0 B [File.ods downloads/file.deb downloads/setup download down do d o w n] true   }" {
					t.Error("Incorrect change object:", yds)
				}
				return
			}
		case <-time.After(time.Second * 10):
			t.Error("No 'idle' status received within 10 sec interval after sync start")
			return
		}
	}
}

func TestError(t *testing.T) {
	_ = exec.Command("yandex-disk", "error").Run()
	select {
	case yds := <-YD.Changes:
		if yds.Stat == "error" {
			if fmt.Sprintf("%v", yds) != "{error idle 43.50 GB 2.88 GB 40.62 GB 654.48 MB [File.ods downloads/file.deb downloads/setup download down do d o w n] false access error downloads/test1 }" {
				t.Error("Incorrect change object:", yds)
			}
			return
		}
		t.Error("Not error status received after error simulation started")
	case <-time.After(time.Second * 2):
		t.Error("No reaction within 2 seconds after error simulation started")
	}
}

func TestStop(t *testing.T) {
	var yds YDvals
	err := YD.Stop()
	if err != nil {
		t.Error("daemon stop error:", err)
	}
	for {
		select {
		case yds = <-YD.Changes:
			if yds.Stat == "none" {
				if fmt.Sprintf("%v", yds) != "{none error     [] true   }" {
					t.Error("Incorrect change object:", yds)
				}
				return
			}
		case <-time.After(time.Second * 3):
			t.Error("'none' status not received within 3 sec interval after daemon stop")
			return
		}
	}
}

func TestSecondaryStop(t *testing.T) {
	err := YD.Stop()
	if err != nil {
		t.Error("daemon stop error:", err)
	}
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
