// Package proc takes point-in-time snapshots of the machine's process table.
package proc

// Proc is one process observed in a snapshot.
type Proc struct {
	PID  int
	PPID int
	RSS  uint64 // resident set size in bytes
	Fair uint64 // fair-share metric in bytes (phys_footprint on macOS, PSS on Linux); 0 if unreadable
	Path string // executable path; empty if unreadable
	Name string // short process name (basename of Path when available)
	Unit string // Linux: innermost cgroup component (e.g. "app-firefox-1234.scope"); empty elsewhere
	// UserUnit is set on Linux when the process lives under a per-user systemd
	// manager (user@UID.service), i.e. its Unit is a `--user` unit.
	UserUnit bool
}

// MemRSS selects the RSS metric for a process.
func MemRSS(p Proc) uint64 { return p.RSS }

// MemFair selects the fair-share metric, falling back to RSS for processes
// the OS wouldn't let us read (typically other users' processes).
func MemFair(p Proc) uint64 {
	if p.Fair > 0 {
		return p.Fair
	}
	return p.RSS
}
