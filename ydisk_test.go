package ydisk

import (
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

func TestStartOk(t *testing.T) {
	// setup yandex-disk
	err := exec.Command("yandex-disk", "token", "-p", "$PASSWD", "$USER").Run()
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
	output := YD.getOutput(true)
	if output == "" {
		t.Error("Empty response from started daemon")
	}
}