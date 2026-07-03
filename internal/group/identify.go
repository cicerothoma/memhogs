package group

import (
	"path/filepath"
	"strings"

	"memhogs/internal/proc"
)

// DarwinHooks identifies applications by the outermost .app bundle in the
// executable path, which resolves renamed binaries (Warp's "stable") and
// folds nested helper bundles into their parent app.
func DarwinHooks() Hooks {
	return Hooks{
		Identify:        func(p proc.Proc) (string, bool) { return outermostBundle(p.Path) },
		IsShellBoundary: isShell,
	}
}

// LinuxHooks identifies applications by systemd cgroup unit: app scopes for
// desktop apps, service units for daemons. Session scopes and slices carry no
// identity — a session must not swallow everything launched inside it.
func LinuxHooks() Hooks {
	return Hooks{
		Identify:        func(p proc.Proc) (string, bool) { return unitIdentity(p.Unit) },
		IsShellBoundary: isShell,
	}
}

func outermostBundle(path string) (string, bool) {
	for _, seg := range strings.Split(path, "/") {
		if name, ok := strings.CutSuffix(seg, ".app"); ok && name != "" {
			return name, true
		}
	}
	return "", false
}

var shellNames = map[string]bool{
	"sh": true, "bash": true, "zsh": true, "fish": true, "dash": true,
	"ash": true, "csh": true, "tcsh": true, "ksh": true, "mksh": true,
	"nu": true, "xonsh": true, "pwsh": true, "elvish": true,
}

func isShell(p proc.Proc) bool {
	name := p.Name
	if p.Path != "" {
		name = filepath.Base(p.Path)
	}
	return shellNames[strings.TrimPrefix(name, "-")] // login shells report as e.g. "-zsh"
}

// launcherPrefixes are wrappers systemd sticks between "app-" and the app id.
var launcherPrefixes = []string{"gnome-", "kde-", "plasma-", "flatpak-", "snap-", "dbus-"}

func unitIdentity(unit string) (string, bool) {
	switch {
	case unit == "" || unit == "init.scope" || strings.HasSuffix(unit, ".slice"):
		return "", false
	case strings.HasSuffix(unit, ".service"):
		name := strings.TrimSuffix(unit, ".service")
		// user@1000.service is the per-user manager, not an app.
		if strings.Contains(name, "@") {
			return "", false
		}
		return name, true
	case strings.HasSuffix(unit, ".scope"):
		name := strings.TrimSuffix(unit, ".scope")
		rest, ok := strings.CutPrefix(name, "app-")
		if !ok {
			return "", false // session-N.scope and friends
		}
		for _, lp := range launcherPrefixes {
			rest = strings.TrimPrefix(rest, lp)
		}
		// Drop the trailing "-<random/pid>" instance suffix.
		if i := strings.LastIndex(rest, "-"); i > 0 && isDigits(rest[i+1:]) {
			rest = rest[:i]
		}
		// Reverse-DNS ids read best by their last component.
		if i := strings.LastIndex(rest, "."); i >= 0 {
			rest = rest[i+1:]
		}
		if rest == "" {
			return "", false
		}
		return rest, true
	}
	return "", false
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
