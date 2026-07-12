package group

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Recipe is how to stop a group: Stop is the idiomatic command (graceful where
// the platform offers it), Force is the hard fallback if Stop doesn't take.
type Recipe struct {
	Stop  string
	Force string
}

// StopRecipe returns the command(s) to stop a group on the given GOOS. It never
// runs anything — memhogs only ever prints these for the user to review.
//
//   - A systemd service is stopped with systemctl, not kill: a unit with
//     Restart=always would just respawn. User services get --user.
//   - A macOS app is quit gracefully via osascript so it can save state.
//   - Everything else is a plain kill of the group's root processes, which
//     takes the whole tree with it.
func StopRecipe(g Group, goos string) Recipe {
	roots := rootPIDs(g)
	kill := func(sig string) string {
		if sig == "" {
			return "kill " + joinInts(roots)
		}
		return "kill -" + sig + " " + joinInts(roots)
	}

	if goos == "linux" && g.Kind == KindApp {
		if unit, user, ok := serviceUnit(g); ok {
			base := "systemctl "
			if user {
				base += "--user "
			}
			return Recipe{Stop: base + "stop " + unit, Force: base + "kill -s KILL " + unit}
		}
	}
	if goos == "darwin" && g.Kind == KindApp {
		return Recipe{Stop: fmt.Sprintf("osascript -e 'quit app %q'", g.Name), Force: kill("9")}
	}
	return Recipe{Stop: kill(""), Force: kill("9")}
}

// rootPIDs returns the PIDs of a group's roots — members whose parent is not
// itself in the group — sorted ascending. Killing these takes their children
// with them. If a PPID cycle leaves no root, every PID is returned so the
// command is still complete.
func rootPIDs(g Group) []int {
	inGroup := make(map[int]bool, len(g.Procs))
	for _, p := range g.Procs {
		inGroup[p.PID] = true
	}
	var roots []int
	for _, p := range g.Procs {
		if !inGroup[p.PPID] {
			roots = append(roots, p.PID)
		}
	}
	if len(roots) == 0 {
		for _, p := range g.Procs {
			roots = append(roots, p.PID)
		}
	}
	sort.Ints(roots)
	return roots
}

// serviceUnit reports the systemd .service unit backing a group, if any, and
// whether it is a per-user service. Scopes (app-*.scope) carry no stoppable
// unit name here — their identity was reduced to a display name — so they fall
// through to a plain kill.
func serviceUnit(g Group) (unit string, user, ok bool) {
	for _, p := range g.Procs {
		if strings.HasSuffix(p.Unit, ".service") {
			return p.Unit, p.UserUnit, true
		}
	}
	return "", false, false
}

func joinInts(pids []int) string {
	ss := make([]string, len(pids))
	for i, p := range pids {
		ss[i] = strconv.Itoa(p)
	}
	return strings.Join(ss, " ")
}
