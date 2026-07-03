//go:build linux

package proc

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Snapshot lists all processes by reading /proc directly: stat for the parent
// PID, statm for resident pages, the exe symlink for the path (readable only
// for the caller's own processes; empty otherwise), and cgroup for the
// systemd unit used by grouping.
func Snapshot() ([]Proc, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	pageSize := uint64(os.Getpagesize())
	var procs []Proc
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		p, ok := readProc(pid, pageSize)
		if ok {
			procs = append(procs, p)
		}
	}
	return procs, nil
}

// TotalMemory returns physical RAM in bytes, or 0 if it can't be determined.
func TotalMemory() uint64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	return parseMemTotal(string(data))
}

func readProc(pid int, pageSize uint64) (Proc, bool) {
	dir := "/proc/" + strconv.Itoa(pid)

	stat, err := os.ReadFile(dir + "/stat")
	if err != nil {
		return Proc{}, false // vanished between listing and read
	}
	name, ppid, ok := parseStat(string(stat))
	if !ok {
		return Proc{}, false
	}

	p := Proc{PID: pid, PPID: ppid, Name: name}

	if statm, err := os.ReadFile(dir + "/statm"); err == nil {
		fields := strings.Fields(string(statm))
		if len(fields) >= 2 {
			if pages, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
				p.RSS = pages * pageSize
			}
		}
	}

	if exe, err := os.Readlink(dir + "/exe"); err == nil {
		p.Path = strings.TrimSuffix(exe, " (deleted)")
		p.Name = filepath.Base(p.Path)
	}

	if cg, err := os.ReadFile(dir + "/cgroup"); err == nil {
		p.Unit = cgroupUnit(string(cg))
	}

	// PSS charges shared pages fractionally, so sums don't double-count.
	// Readable only for the caller's own processes (all of them as root);
	// missing on pre-4.14 kernels. 0 means "fall back to RSS".
	if sm, err := os.ReadFile(dir + "/smaps_rollup"); err == nil {
		p.Fair = parsePss(string(sm))
	}
	return p, true
}
