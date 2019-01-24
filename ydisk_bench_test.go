package ydisk

import (
	"regexp"
	"strings"
	"testing"
)

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
	split := []string{out[:n], out[n+25:]}
	// Need to remove "Path to " as another "Path:" exists in case of access error
	split[0] = strings.Replace(split[0], "Path to ", "", 1)
	// Initialize map with keys that can be missed
	keys := map[string]string{"Sync": "", "Error": "", "Path": ""}
	// Take only first word in the phrase before ":"
	for _, s := range regexp.MustCompile(`\s*([^ ]+).*: (.*)`).FindAllStringSubmatch(split[0], -1) {
		if s[2][0] == byte('\'') {
			s[2] = s[2][1 : len(s[2])-1] // remove ' in the begging and at end
		}
		keys[s[1]] = s[2]
	}
	// map representation of switch_case clause
	for k, v := range map[string]*string{
		"Synchronization": &val.Stat,
		"Total":           &val.Total,
		"Used":            &val.Used,
		"Available":       &val.Free,
		"Trash":           &val.Trash,
		"Error":           &val.Err,
		"Path":            &val.ErrP,
		"Sync":            &val.Prog,
	} {
		setChanged(v, keys[k], &changed)
	}
	// Parse the "Last synchronized items" section (list of paths and files)
	val.ChLast = false // track last list changes separately
	if len(split) > 1 {
		f := regexp.MustCompile(`: '(.*)'\n`).FindAllStringSubmatch(split[1], -1)
		if len(f) != len(val.Last) {
			val.ChLast = true
			val.Last = []string{}
			for _, p := range f {
				val.Last = append(val.Last, p[1])
			}
		} else {
			for i, p := range f {
				setChanged(&val.Last[i], p[1], &val.ChLast)
			}
		}
	} else { // len(split) = 1 - there is no section with last sync. paths
		if len(val.Last) > 0 {
			val.Last = []string{}
			val.ChLast = true
		}
	}
	return changed || val.ChLast
}

func BenchmarkYDvalUpdate(b *testing.B) {
	st1 := "Sync progress: 139.38 MB/ 139.38 MB (100 %)\nSynchronization core status: index\nPath to Yandex.Disk directory: '/home/stc/Yandex.Disk'\n\tTotal: 43.50 GB\n\tUsed: 2.89 GB\n\tAvailable: 40.61 GB\n\tMax file size: 50 GB\n\tTrash size: 0 B\n\nLast synchronized items:\n\tfile: 'NewFile'\n\tfile: 'File.ods'\n\tfile: 'downloads/file.deb'\n\tfile: 'downloads/setup'\n\tfile: 'download'\n\tfile: 'down'\n\tfile: 'do'\n\tfile: 'd'\n\tfile: 'o'\n\tfile: 'w'\n\n"
	//st2 := "Synchronization core status: idle\nPath to Yandex.Disk directory: '/home/stc/Yandex.Disk'\n\tTotal: 43.50 GB\n\tUsed: 2.89 GB\n\tAvailable: 40.61 GB\n\tMax file size: 50 GB\n\tTrash size: 0 B\n\nLast synchronized items:\n\tfile: 'File.ods'\n\tfile: 'downloads/file.deb'\n\tfile: 'downloads/setup'\n\tfile: 'download'\n\tfile: 'down'\n\tfile: 'do'\n\tfile: 'd'\n\tfile: 'o'\n\tfile: 'w'\n\tfile: 'n'\n\n"
	for i := 0; i < b.N; i++ {
		yd := newYDvals()
		yd.update(st1)
	}
}
func BenchmarkYDvalUpdate1(b *testing.B) {
	st1 := "Sync progress: 139.38 MB/ 139.38 MB (100 %)\nSynchronization core status: index\nPath to Yandex.Disk directory: '/home/stc/Yandex.Disk'\n\tTotal: 43.50 GB\n\tUsed: 2.89 GB\n\tAvailable: 40.61 GB\n\tMax file size: 50 GB\n\tTrash size: 0 B\n\nLast synchronized items:\n\tfile: 'NewFile'\n\tfile: 'File.ods'\n\tfile: 'downloads/file.deb'\n\tfile: 'downloads/setup'\n\tfile: 'download'\n\tfile: 'down'\n\tfile: 'do'\n\tfile: 'd'\n\tfile: 'o'\n\tfile: 'w'\n\n"
	//st2 := "Synchronization core status: idle\nPath to Yandex.Disk directory: '/home/stc/Yandex.Disk'\n\tTotal: 43.50 GB\n\tUsed: 2.89 GB\n\tAvailable: 40.61 GB\n\tMax file size: 50 GB\n\tTrash size: 0 B\n\nLast synchronized items:\n\tfile: 'File.ods'\n\tfile: 'downloads/file.deb'\n\tfile: 'downloads/setup'\n\tfile: 'download'\n\tfile: 'down'\n\tfile: 'do'\n\tfile: 'd'\n\tfile: 'o'\n\tfile: 'w'\n\tfile: 'n'\n\n"
	for i := 0; i < b.N; i++ {
		yd := newYDvals()
		yd.update1(st1)
	}
}
