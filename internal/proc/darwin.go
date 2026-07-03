//go:build darwin

package proc

import (
	"os/exec"
	"strconv"
	"strings"
)

// Snapshot lists all processes via ps. On macOS, `comm` prints the full
// executable path, which is what bundle resolution needs; rss is in KiB.
func Snapshot() ([]Proc, error) {
	out, err := exec.Command("ps", "axo", "pid=,ppid=,rss=,comm=").Output()
	if err != nil {
		return nil, err
	}
	procs, err := parsePS(out)
	if err != nil {
		return nil, err
	}
	for i := range procs {
		procs[i].Fair = footprint(procs[i].PID)
	}
	return procs, nil
}

// TotalMemory returns physical RAM in bytes, or 0 if it can't be determined.
func TotalMemory() uint64 {
	out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 0
	}
	n, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0
	}
	return n
}
