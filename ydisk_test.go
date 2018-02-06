package ydisk

import (
	"os/exec"
	"os"
	"path/filepath"
	"testing"

	"github.com/slytomcat/llog"
)

func init() {
	llog.SetLevel(llog.DEBUG)
}

func TestFailInit(t *testing.T) {
	home := os.Getenv("HOME")
	cfg := filepath.Join(home, ".config", "yandex-disk", "config.cfg")
	// look for installed yandex-disk daemon
	daemon, err  := exec.LookPath("yandex-disk")
	notInstalled := true
	if err == nil {
		llog.Info("yandex-disk installed. Try to rename it for not_installed case test")
		err = os.Rename(daemon, daemon+"_")
		if err != nil {
			llog.Error("Can't rename yandex-disk: NOT_INSTALLED case can't be tested")
			notInstalled = false
		}
	}	

	if notInstalled {
		// test not_installed case
		llog.Info("yandex-disk daemon is not installed")
		_, err = NewYDisk(cfg)
		if err == nil {
			t.Error("Initialized with not installed daemon")
		}
	}
	// restore daemon it it was renamed before
	if daemon != "" && notInstalled {
		_ = os.Rename(daemon+"_", daemon)
	}

	// test initialization with wrong config 
	_, err = NewYDisk(cfg+"_bad")
	if err == nil {
		t.Error("Initialized with not existing daemon config file")
	}

}