/*
Package ydisk implements API for yandex-disk daemon. Logging is organized
via github.com/slytomcat/llog package.
*/
package ydisk

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/slytomcat/llog"
)

// YDvals - Daemon Status structure
type YDvals struct {
	Stat   string         // Current Status
	Prev   string         // Previous Status
	Total  string         // Total space available
	Used   string         // Used space
	Free   string         // Free space
	Trash  string         // Trash size
	Last   []string       // Last-updated files/folders list (10 or less items)
	ChLast bool           // Indicator that Last was changed
	Err    string         // Error status message
	ErrP   string         // Error path
	Prog   string         // Synchronization progress (when in busy status)
}

/* A new YDvals constsructor */
func newYDvals() YDvals {
	return YDvals{
		"unknown",      // Current Status
		"unknown",      // Previous Status
		"", "", "", "", // Total, Used, Free, Trash
		[]string{}, // Last
		false,      // ChLast
		"", "", "", // Err, ErrP, Prog
	}
}

/* Tool function that controls the change of value in variable */
func setChanged(v *string, val string, c *bool) {
	if *v != val {
		*v = val
		*c = true
	}
}

/* update - Updates Daemon status values from the daemon output string.
   Returns true if a change detected in any value, otherwise returns false */
func (val *YDvals) update(out string) bool {
	val.Prev = val.Stat // store previous status but don't track changes of val.Prev
	changed := false    // track changes for values
	if out == "" {
		setChanged(&val.Stat, "none", &changed)
		if changed {
			val.Total, val.Used, val.Trash, val.Free = "", "", "", ""
			val.Prog, val.Err, val.ErrP, val.ChLast = "", "", "", true
			val.Last = []string{}
		}
		return changed
	}
	n := strings.Index(out, "Last synchronized items:")
	val.ChLast = false // track last list changes separately
	if n > 0 {
		// Parse the "Last synchronized items" section (list of paths and files)
		f := make([]string, 0, 10)
		for _, s := range strings.Split(out[n+24:], "\n") {
			if s != "" {
				f = append(f, s[strings.Index(s, ":")+3:len(s)-1])
			}
		}
		if len(f) != len(val.Last) {
			val.ChLast = true
			val.Last = f
		} else {
			for i, p := range f {
				setChanged(&val.Last[i], p, &val.ChLast)
			}
		}
	} else {
		n = len(out)
		if len(val.Last) > 0 {
			val.Last = []string{}
			val.ChLast = true
		}
	}

	// Initialize map with keys that can be missed
	keys := map[string]string{"Sync progress": "", "Error": "", "Path": ""}
	for _, s := range strings.Split(out[:n], "\n") {
		if n := strings.Index(s, ":"); n > 0 {
			keys[strings.TrimSpace(s[:n])] = s[n+2:]
		}
	}
	for k, v := range keys {
		switch k {
		case "Synchronization core status":
			setChanged(&val.Stat, v, &changed)
		case "Total":
			setChanged(&val.Total, v, &changed)
		case "Used":
			setChanged(&val.Used, v, &changed)
		case "Available":
			setChanged(&val.Free, v, &changed)
		case "Trash size":
			setChanged(&val.Trash, v, &changed)
		case "Sync progress":
			setChanged(&val.Prog, v, &changed)
		case "Error":
			setChanged(&val.Err, v, &changed)
		case "Path":
			if v != "" {
				setChanged(&val.ErrP, v[1:len(v)-1], &changed)
			} else {
				setChanged(&val.ErrP, "", &changed)
			}
		}
	}
	return changed || val.ChLast
}

type watcher struct {
	*fsnotify.Watcher
	active bool // Flag that means that watching path was successfully added
}

func newwatcher() watcher {
	watch, err := fsnotify.NewWatcher()
	if err != nil {
		llog.Critical(err)
	}
	return watcher{
		watch,
		false,
	}
}

func (w *watcher) activate(path string) {
	if !w.active {
		err := w.Add(filepath.Join(path, ".sync/cli.log"))
		if err != nil {
			llog.Debug("Watch path error:", err)
			return
		}
		llog.Debug("Watch path added")
		w.active = true
	}
}

// YDisk provides methods to interact with yandex-disk (methods: Start, Stop, Output), path
// of synchronized catalogue (property Path) and channel for receiving yandex-disk status
// changes (property Changes).
type YDisk struct {
	Path     string      // Path to synchronized folder (obtained from yandex-disk conf. file)
	Changes  chan YDvals // Output channel for detected changes in daemon status
	conf     string      // Path to yandex-disc configuration file
	exe      string      // Path to yandex-disk executable
	exit     chan bool   // Stop signal/replay channel for Event handler routine
	activate func()      // Function to activate watcher after daemon creation
}

// NewYDisk creates new YDisk structure for communication with yandex-disk daemon
// Parameter:
//  conf - full path to yandex-disk daemon configuration file
//
// Checks performed in the beginning:
//
//  - check that yandex-disk was installed
//  - check that yandex-disk was properly configured
//
// When something not good NewYDisk returns not nil error
func NewYDisk(conf string) (*YDisk, error) {
	path, err := checkDaemon(conf)
	if err != nil {
		return nil, err
	}
	watch := newwatcher()
	exe, err := exec.LookPath("yandex-disk")
	if err != nil {
		return nil, err
	}
	llog.Debug("yandex-disk executable is:", exe)
	yd := YDisk{
		path,
		make(chan YDvals, 1), // Output should be buffered
		conf,
		exe,
		make(chan bool),
		func() { watch.activate(path) },
	}
	// start event handler in separate goroutine
	go yd.eventHandler(watch)
	// Try to activate watching at the beginning. It may fail but it is not a problem
	// as it can be activated later (on Start of daemon).
	yd.activate()
	llog.Debug("New YDisk created and initialized. Path:", path)
	return &yd, nil
}

// eventHandler works in separate goroutine until YDisk.exit channel receives a bool value (any).
func (yd *YDisk) eventHandler(watch watcher) {
	llog.Debug("Event handler started")
	yds := newYDvals()
	interval := 1
	tick := time.NewTimer(time.Millisecond * 100) // First time trigger it quickly to update the current status
	defer func() {
		watch.Close()
		tick.Stop()
		close(yd.Changes)
		llog.Debug("Event handler exited")
		yd.exit <- true // Report exit completion
	}()
	for {
		select {
		case err := <-watch.Errors:
			llog.Error("Watcher error:", err)
			return
		case <-yd.exit:
			return
		case event := <-watch.Events:
			llog.Debug("Watcher event:", event)
			interval = 1
		case <-tick.C:
			llog.Debug("Timer interval:", interval)
			if yds.Stat == "busy" || yds.Stat == "index" {
				interval = 2 // keep 2s interval in busy mode
			} else {
				interval <<= 1 // continuously increase timer interval: 2s, 4s, 8s.
			}
		}
		// in both cases (Timer or Watcher events):
		//  - restart timer
		if interval < 10 {
			tick.Reset(time.Duration(interval) * time.Second)
		}
		//  - check for daemon changes and send changed values in case of change
		if yds.update(yd.getOutput(false)) {
			llog.Debug("Change: ", yds.Prev, ">", yds.Stat,
				"S", len(yds.Total) > 0, "L", len(yds.Last), "E", len(yds.Err) > 0)
			yd.Changes <- yds
		}
		//llog.Debug("Event processed")
	}
}

func (yd YDisk) getOutput(userLang bool) string {
	cmd := []string{"env", "-i", yd.exe, "status", "-c", yd.conf}
	if userLang {
		cmd = cmd[2:]
	}
	out, err := exec.Command(cmd[0], cmd[1:]...).Output()
	if err != nil {
		llog.Debug("daemon status error:" + err.Error())
		return ""
	}
	return string(out)
}

// Close deactivates the daemon connection: stops event handler that closes file watcher
// and Changes channel.
func (yd *YDisk) Close() {
	yd.exit <- true
	<-yd.exit // Wait for the event handler completion
}

// Output returns the output string of `yandex-disk status` command in the current user language.
func (yd *YDisk) Output() string {
	return yd.getOutput(true)
}

// Start runs `yandex-disk start` if daemon was not started before.
func (yd *YDisk) Start() error {
	if yd.getOutput(true) == "" {
		out, err := exec.Command(yd.exe, "start", "-c", yd.conf).Output()
		if err != nil {
			llog.Error(err)
			return err
		}
		llog.Debugf("Daemon start: %s", bytes.TrimRight(out, " \n"))
	} else {
		llog.Debug("Daemon already started")
	}
	yd.activate() // try to activate watching after daemon start. It shouldn't fail on started daemon
	return nil
}

// Stop runs `yandex-disk stop` if daemon was not stopped before.
func (yd *YDisk) Stop() error {
	if yd.getOutput(true) != "" {
		out, err := exec.Command(yd.exe, "stop", "-c", yd.conf).Output()
		if err != nil {
			llog.Error(err)
			return err
		}
		llog.Debugf("Daemon stop: %s", bytes.TrimRight(out, " \n"))
	} else {
		llog.Debug("Daemon already stopped")
	}
	return nil
}
