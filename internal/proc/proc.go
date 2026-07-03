// Package proc takes point-in-time snapshots of the machine's process table.
package proc

// Proc is one process observed in a snapshot.
type Proc struct {
	PID  int
	PPID int
	RSS  uint64 // resident set size in bytes
	Path string // executable path; empty if unreadable
	Name string // short process name (basename of Path when available)
	Unit string // Linux: innermost cgroup component (e.g. "app-firefox-1234.scope"); empty elsewhere
}
