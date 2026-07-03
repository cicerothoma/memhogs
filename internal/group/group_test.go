package group

import (
	"testing"

	"memhogs/internal/proc"
)

const MiB = 1 << 20

// fixture mirrors the process tree from the original investigation session:
// Electron helper fragmentation (Dia), a renamed binary (Warp's "stable"),
// a tsserver spawned by Cursor's extension host with no .app in its path,
// and a user-launched python job under a Warp shell that must NOT roll into Warp.
func darwinFixture() []proc.Proc {
	return []proc.Proc{
		{PID: 1, PPID: 0, RSS: 30 * MiB, Path: "/sbin/launchd", Name: "launchd"},

		// Dia: main proc + helpers, all inside Dia.app (helpers in nested bundles).
		{PID: 100, PPID: 1, RSS: 500 * MiB, Path: "/Applications/Dia.app/Contents/MacOS/Dia", Name: "Dia"},
		{PID: 101, PPID: 100, RSS: 7000 * MiB, Path: "/Applications/Dia.app/Contents/Frameworks/Dia Helper (Renderer).app/Contents/MacOS/Dia Helper (Renderer)", Name: "Dia Helper (Renderer)"},
		{PID: 102, PPID: 100, RSS: 300 * MiB, Path: "/Applications/Dia.app/Contents/Frameworks/Dia Helper (GPU).app/Contents/MacOS/Dia Helper (GPU)", Name: "Dia Helper (GPU)"},

		// Warp: binary literally named "stable"; login shell rolls in,
		// but the python job launched from that shell must not.
		{PID: 200, PPID: 1, RSS: 800 * MiB, Path: "/Applications/Warp.app/Contents/MacOS/stable", Name: "stable"},
		{PID: 201, PPID: 200, RSS: 10 * MiB, Path: "/bin/zsh", Name: "-zsh"},
		{PID: 202, PPID: 201, RSS: 4000 * MiB, Path: "/usr/local/bin/python3", Name: "python3"},
		{PID: 203, PPID: 202, RSS: 1000 * MiB, Path: "/usr/local/bin/python3", Name: "python3"}, // multiprocessing worker

		// Cursor: extension host spawns tsserver via node — no .app in its own path.
		{PID: 300, PPID: 1, RSS: 400 * MiB, Path: "/Applications/Cursor.app/Contents/MacOS/Cursor", Name: "Cursor"},
		{PID: 301, PPID: 300, RSS: 900 * MiB, Path: "/Applications/Cursor.app/Contents/Frameworks/Cursor Helper (Plugin).app/Contents/MacOS/Cursor Helper (Plugin)", Name: "Cursor Helper (Plugin)"},
		{PID: 302, PPID: 301, RSS: 117 * MiB, Path: "/usr/local/bin/node", Name: "node"},

		// Daemon straight under launchd: stands alone, doesn't merge into a launchd blob.
		{PID: 400, PPID: 1, RSS: 100 * MiB, Path: "/opt/homebrew/opt/mysql/bin/mysqld", Name: "mysqld"},

		// Same-named system daemons merge into one row by name.
		{PID: 500, PPID: 1, RSS: 50 * MiB, Path: "/System/Library/Frameworks/CoreServices.framework/mdworker_shared", Name: "mdworker_shared"},
		{PID: 501, PPID: 1, RSS: 50 * MiB, Path: "/System/Library/Frameworks/CoreServices.framework/mdworker_shared", Name: "mdworker_shared"},

		// Zero-RSS entries are dropped as noise.
		{PID: 600, PPID: 1, RSS: 0, Path: "/usr/libexec/idle_thing", Name: "idle_thing"},

		// Orphan whose parent already exited: stands alone, no crash.
		{PID: 700, PPID: 699, RSS: 20 * MiB, Path: "/usr/local/bin/orphan", Name: "orphan"},
	}
}

func byName(t *testing.T, groups []Group, name string) Group {
	t.Helper()
	for _, g := range groups {
		if g.Name == name {
			return g
		}
	}
	t.Fatalf("no group named %q; got %v", name, names(groups))
	return Group{}
}

func names(groups []Group) []string {
	out := make([]string, len(groups))
	for i, g := range groups {
		out[i] = g.Name
	}
	return out
}

func TestDarwinGrouping(t *testing.T) {
	groups := Build(darwinFixture(), DarwinHooks(), proc.MemRSS)

	tests := []struct {
		name  string
		mem   uint64
		procs int
		kind  Kind
	}{
		{"Dia", 7800 * MiB, 3, KindApp},                   // helpers aggregate under outermost bundle
		{"Warp", 810 * MiB, 2, KindApp},                   // "stable" resolved to Warp; login shell rolls in
		{"python3", 5000 * MiB, 2, KindStandalone},        // stopped at shell; worker rolls into the job
		{"Cursor", 1417 * MiB, 3, KindApp},                // tsserver reaches Cursor via ancestry, not path
		{"mysqld", 100 * MiB, 1, KindStandalone},          // direct child of launchd stands alone
		{"mdworker_shared", 100 * MiB, 2, KindStandalone}, // same-named standalones merge
		{"orphan", 20 * MiB, 1, KindStandalone},
	}
	for _, tt := range tests {
		g := byName(t, groups, tt.name)
		if g.Mem != tt.mem {
			t.Errorf("%s: Mem = %d MiB, want %d MiB", tt.name, g.Mem/MiB, tt.mem/MiB)
		}
		if len(g.Procs) != tt.procs {
			t.Errorf("%s: %d procs, want %d", tt.name, len(g.Procs), tt.procs)
		}
		if g.Kind != tt.kind {
			t.Errorf("%s: kind = %v, want %v", tt.name, g.Kind, tt.kind)
		}
	}

	for _, g := range groups {
		if g.Name == "idle_thing" {
			t.Error("zero-RSS process should be dropped")
		}
	}
}

func TestSortedByRSSDescending(t *testing.T) {
	groups := Build(darwinFixture(), DarwinHooks(), proc.MemRSS)
	for i := 1; i < len(groups); i++ {
		if groups[i].Mem > groups[i-1].Mem {
			t.Fatalf("groups not sorted: %s (%d) after %s (%d)",
				groups[i].Name, groups[i].Mem, groups[i-1].Name, groups[i-1].Mem)
		}
	}
}

func TestFairMetricPreferredWithRSSFallback(t *testing.T) {
	procs := []proc.Proc{
		// Fair known: charged at Fair, not RSS.
		{PID: 100, PPID: 1, RSS: 500 * MiB, Fair: 300 * MiB, Path: "/Applications/Dia.app/Contents/MacOS/Dia", Name: "Dia"},
		// Fair unreadable: falls back to RSS.
		{PID: 101, PPID: 100, RSS: 200 * MiB, Fair: 0, Path: "/Applications/Dia.app/Contents/Frameworks/Dia Helper.app/Contents/MacOS/Dia Helper", Name: "Dia Helper"},
	}
	groups := Build(procs, DarwinHooks(), proc.MemFair)
	g := byName(t, groups, "Dia")
	if g.Mem != 500*MiB {
		t.Errorf("Dia: Mem = %d MiB, want 500 (300 fair + 200 rss fallback)", g.Mem/MiB)
	}
}

func TestCycleInParentChainDoesNotHang(t *testing.T) {
	procs := []proc.Proc{
		{PID: 10, PPID: 11, RSS: MiB, Name: "a"},
		{PID: 11, PPID: 10, RSS: MiB, Name: "b"},
	}
	groups := Build(procs, DarwinHooks(), proc.MemRSS) // must terminate
	if len(groups) == 0 {
		t.Fatal("expected groups from cyclic fixture")
	}
}

func TestLinuxGrouping(t *testing.T) {
	procs := []proc.Proc{
		{PID: 1, PPID: 0, RSS: 10 * MiB, Path: "/usr/lib/systemd/systemd", Name: "systemd", Unit: "init.scope"},

		// Firefox under an app scope: all procs share the unit.
		{PID: 900, PPID: 1, RSS: 600 * MiB, Path: "/usr/lib/firefox/firefox", Name: "firefox", Unit: "app-gnome-firefox-4321.scope"},
		{PID: 901, PPID: 900, RSS: 400 * MiB, Path: "/usr/lib/firefox/firefox", Name: "Isolated Web Co", Unit: "app-gnome-firefox-4321.scope"},

		// System service.
		{PID: 950, PPID: 1, RSS: 80 * MiB, Path: "/usr/sbin/sshd", Name: "sshd", Unit: "ssh.service"},

		// SSH session: session scope must NOT become a group that swallows the user's job;
		// the shell boundary makes the python job stand alone.
		{PID: 960, PPID: 950, RSS: 8 * MiB, Path: "/usr/bin/bash", Name: "bash", Unit: "session-3.scope"},
		{PID: 961, PPID: 960, RSS: 2000 * MiB, Path: "/usr/bin/python3", Name: "python3", Unit: "session-3.scope"},

		// Kernel thread: no RSS, dropped.
		{PID: 2, PPID: 0, RSS: 0, Name: "kthreadd"},
	}
	groups := Build(procs, LinuxHooks(), proc.MemRSS)

	if g := byName(t, groups, "firefox"); g.Mem != 1000*MiB || len(g.Procs) != 2 || g.Kind != KindApp {
		t.Errorf("firefox: got RSS %d MiB, %d procs, kind %v", g.Mem/MiB, len(g.Procs), g.Kind)
	}
	if g := byName(t, groups, "ssh"); g.Kind != KindApp {
		t.Errorf("ssh: want service identity via ssh.service, got kind %v", g.Kind)
	}
	if g := byName(t, groups, "python3"); g.Mem != 2000*MiB || g.Kind != KindStandalone {
		t.Errorf("python3: want standalone 2000 MiB, got %d MiB kind %v", g.Mem/MiB, g.Kind)
	}
	for _, g := range groups {
		if g.Name == "session-3" || g.Name == "session-3.scope" {
			t.Error("session scope must not become a group identity")
		}
	}
}
