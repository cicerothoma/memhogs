package proc

// Pure parsers for /proc file contents, kept outside the linux build tag so
// they compile and test on every platform.

import (
	"path/filepath"
	"strconv"
	"strings"
)

// parseStat extracts comm and ppid from /proc/pid/stat, whose format is
// "pid (comm) state ppid ..." — comm itself may contain spaces and parens,
// so scan from the last ')'.
func parseStat(stat string) (name string, ppid int, ok bool) {
	open := strings.IndexByte(stat, '(')
	close := strings.LastIndexByte(stat, ')')
	if open < 0 || close < open {
		return "", 0, false
	}
	fields := strings.Fields(stat[close+1:])
	if len(fields) < 2 {
		return "", 0, false
	}
	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		return "", 0, false
	}
	return stat[open+1 : close], ppid, true
}

// cgroupUnit returns the innermost component of the process's cgroup v2 path
// (falling back to the first line for hybrid hierarchies), e.g.
// "0::/system.slice/ssh.service" -> "ssh.service".
func cgroupUnit(cg string) string {
	var path string
	for line := range strings.Lines(cg) {
		line = strings.TrimSpace(line)
		rest, found := strings.CutPrefix(line, "0::")
		if found {
			path = rest
			break
		}
		if path == "" {
			if parts := strings.SplitN(line, ":", 3); len(parts) == 3 {
				path = parts[2]
			}
		}
	}
	if path == "" || path == "/" {
		return ""
	}
	return filepath.Base(path)
}

// cgroupUserScoped reports whether the cgroup v2 path runs under a per-user
// systemd manager (user@UID.service), which means its units are `--user` units
// stopped with `systemctl --user`. System units live under system.slice.
func cgroupUserScoped(cg string) bool {
	for line := range strings.Lines(cg) {
		line = strings.TrimSpace(line)
		rest, found := strings.CutPrefix(line, "0::")
		if !found {
			if parts := strings.SplitN(line, ":", 3); len(parts) == 3 {
				rest = parts[2]
			}
		}
		if strings.Contains(rest, "/user@") {
			return true
		}
	}
	return false
}

// parsePss extracts the proportional set size in bytes from
// /proc/pid/smaps_rollup content ("Pss:            123456 kB").
func parsePss(rollup string) uint64 {
	for line := range strings.Lines(rollup) {
		rest, found := strings.CutPrefix(line, "Pss:")
		if !found {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) < 1 {
			return 0
		}
		kb, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return 0
		}
		return kb * 1024
	}
	return 0
}

// parseMemTotal extracts physical RAM in bytes from /proc/meminfo content
// ("MemTotal:       65486788 kB").
func parseMemTotal(meminfo string) uint64 {
	for line := range strings.Lines(meminfo) {
		rest, found := strings.CutPrefix(line, "MemTotal:")
		if !found {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) < 1 {
			return 0
		}
		kb, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return 0
		}
		return kb * 1024
	}
	return 0
}
