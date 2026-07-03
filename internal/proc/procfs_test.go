package proc

import "testing"

func TestParseStat(t *testing.T) {
	tests := []struct {
		stat string
		name string
		ppid int
		ok   bool
	}{
		{"302 (tsserver[5.9.2]) S 301 302 300 0 -1", "tsserver[5.9.2]", 301, true},
		{"42 (evil ) name) R 1 42 42 0 -1", "evil ) name", 1, true}, // parens in comm
		{"1 (systemd) S 0 1 1", "systemd", 0, true},
		{"garbage", "", 0, false},
		{"", "", 0, false},
	}
	for _, tt := range tests {
		name, ppid, ok := parseStat(tt.stat)
		if name != tt.name || ppid != tt.ppid || ok != tt.ok {
			t.Errorf("parseStat(%q) = %q,%d,%v want %q,%d,%v", tt.stat, name, ppid, ok, tt.name, tt.ppid, tt.ok)
		}
	}
}

func TestCgroupUnit(t *testing.T) {
	tests := []struct {
		cg   string
		want string
	}{
		{"0::/user.slice/user-1000.slice/user@1000.service/app.slice/app-gnome-firefox-4321.scope\n", "app-gnome-firefox-4321.scope"},
		{"0::/system.slice/ssh.service\n", "ssh.service"},
		{"0::/\n", ""},
		{"12:pids:/system.slice/cron.service\n1:name=systemd:/system.slice/cron.service\n", "cron.service"}, // cgroup v1 fallback
		{"", ""},
	}
	for _, tt := range tests {
		if got := cgroupUnit(tt.cg); got != tt.want {
			t.Errorf("cgroupUnit(%q) = %q, want %q", tt.cg, got, tt.want)
		}
	}
}
