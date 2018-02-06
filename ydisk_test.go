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
	_, err  := exec.LookPath("yandex-disk")
	if err != nil {
		// test not_installed case
		llog.Info("yandex-disk daemon is not installed")
		_, err = NewYDisk(cfg)
		if err == nil {
			t.Error("Initialized with not installed daemon")
		}
		// install yandex-disk
		err = exec.Command(`echo "deb http://repo.yandex.ru/yandex-disk/deb/ stable main" | sudo tee -a /etc/apt/sources.list.d/yandex.list > /dev/null && wget http://repo.yandex.ru/yandex-disk/YANDEX-DISK-KEY.GPG -O- | sudo apt-key add - && sudo apt-get update && sudo apt-get install -y yandex-disk`).Run()
		if err != nil {
			t.Error("Can't install daemon")
		}
	} else {
		llog.Info("yandex-disk daemon is installed: Test_NOT_INSTALLED is skipped")
	}

	// test initialization with wrong config 
	_, err = NewYDisk(cfg+"_bad")
	if err == nil {
		t.Error("Initialized with not existing daemon config file")
	}
}