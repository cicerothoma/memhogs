// Package group rolls a process snapshot up into per-application groups.
//
// The engine is platform-agnostic: it walks each process's parent chain until
// a platform hook recognizes an owning application, stopping early when the
// chain would cross an interactive shell (things launched from a shell are
// their own group, not the terminal's).
package group

import (
	"sort"

	"memhogs/internal/proc"
)

type Kind int

const (
	// KindApp is a group anchored to a recognized application or service
	// (a .app bundle on macOS, a systemd app scope / service unit on Linux).
	KindApp Kind = iota
	// KindStandalone is a group rooted at a process with no recognized owner,
	// merged by root process name.
	KindStandalone
)

func (k Kind) String() string {
	if k == KindApp {
		return "app"
	}
	return "standalone"
}

type Group struct {
	Name  string
	Kind  Kind
	Mem   uint64 // sum of the selected metric over Procs
	Procs []proc.Proc
}

// Hooks are the two platform-specific decisions the engine defers.
type Hooks struct {
	// Identify reports the owning application of a single process, if the
	// platform can tell from the process alone (bundle path, cgroup unit).
	Identify func(proc.Proc) (string, bool)
	// IsShellBoundary reports whether a process is an interactive shell that
	// should stop the ancestry walk.
	IsShellBoundary func(proc.Proc) bool
}

const maxWalk = 128 // parent chains are shallow; this only guards PPID cycles

// Build assigns every process to a group and returns groups sorted by the
// selected metric descending (ties by name). Zero-memory processes are
// dropped as noise.
func Build(procs []proc.Proc, h Hooks, mem func(proc.Proc) uint64) []Group {
	byPID := make(map[int]proc.Proc, len(procs))
	for _, p := range procs {
		byPID[p.PID] = p
	}

	groups := make(map[string]*Group)
	assign := func(key, name string, kind Kind, p proc.Proc) {
		g, ok := groups[key]
		if !ok {
			g = &Group{Name: name, Kind: kind}
			groups[key] = g
		}
		g.Mem += mem(p)
		g.Procs = append(g.Procs, p)
	}

	for _, p := range procs {
		if mem(p) == 0 {
			continue
		}
		name, kind := resolve(p, byPID, h)
		assign(kind.String()+":"+name, name, kind, p)
	}

	out := make([]*Group, 0, len(groups))
	for _, g := range groups {
		sort.Slice(g.Procs, func(i, j int) bool { return mem(g.Procs[i]) > mem(g.Procs[j]) })
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Mem != out[j].Mem {
			return out[i].Mem > out[j].Mem
		}
		return out[i].Name < out[j].Name
	})

	result := make([]Group, len(out))
	for i, g := range out {
		result[i] = *g
	}
	return result
}

// resolve finds the group a process belongs to. Walking upward from p:
// the first ancestor (or p itself) with a platform identity claims it;
// hitting a shell, PID 1, or a missing/cyclic parent makes it standalone,
// named after the topmost ancestor reached below that boundary.
func resolve(p proc.Proc, byPID map[int]proc.Proc, h Hooks) (string, Kind) {
	if name, ok := h.Identify(p); ok {
		return name, KindApp
	}
	cur := p
	for range maxWalk {
		parent, ok := byPID[cur.PPID]
		if !ok || parent.PID == cur.PID || parent.PID <= 1 {
			break
		}
		if h.IsShellBoundary(parent) {
			break
		}
		if name, ok := h.Identify(parent); ok {
			return name, KindApp
		}
		cur = parent
	}
	return cur.Name, KindStandalone
}
