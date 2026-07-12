package group

import (
	"testing"

	"github.com/cicerothoma/memhogs/internal/proc"
)

func TestStopRecipe(t *testing.T) {
	// A three-process app tree: root 100, children 101 and 102.
	appTree := []proc.Proc{
		{PID: 100, PPID: 1, Name: "Dia"},
		{PID: 101, PPID: 100, Name: "Helper"},
		{PID: 102, PPID: 101, Name: "Renderer"},
	}
	svc := func(unit string, user bool) []proc.Proc {
		return []proc.Proc{{PID: 200, PPID: 1, Unit: unit, UserUnit: user}}
	}

	tests := []struct {
		name  string
		g     Group
		goos  string
		stop  string
		force string
	}{
		{"darwin app quits gracefully", Group{Name: "Dia", Kind: KindApp, Procs: appTree}, "darwin",
			`osascript -e 'quit app "Dia"'`, "kill -9 100"},
		{"darwin standalone kills root", Group{Name: "node", Kind: KindStandalone, Procs: appTree}, "darwin",
			"kill 100", "kill -9 100"},
		{"linux system service", Group{Name: "docker", Kind: KindApp, Procs: svc("docker.service", false)}, "linux",
			"systemctl stop docker.service", "systemctl kill -s KILL docker.service"},
		{"linux user service", Group{Name: "pipewire", Kind: KindApp, Procs: svc("pipewire.service", true)}, "linux",
			"systemctl --user stop pipewire.service", "systemctl --user kill -s KILL pipewire.service"},
		{"linux app scope kills root", Group{Name: "firefox", Kind: KindApp, Procs: []proc.Proc{
			{PID: 300, PPID: 1, Unit: "app-firefox-1.scope"}}}, "linux",
			"kill 300", "kill -9 300"},
		{"linux standalone kills root", Group{Name: "python3", Kind: KindStandalone, Procs: appTree}, "linux",
			"kill 100", "kill -9 100"},
	}
	for _, tt := range tests {
		got := StopRecipe(tt.g, tt.goos)
		if got.Stop != tt.stop {
			t.Errorf("%s: Stop = %q, want %q", tt.name, got.Stop, tt.stop)
		}
		if got.Force != tt.force {
			t.Errorf("%s: Force = %q, want %q", tt.name, got.Force, tt.force)
		}
	}
}

func TestRootPIDs(t *testing.T) {
	tests := []struct {
		name  string
		procs []proc.Proc
		want  []int
	}{
		{"single root with descendants", []proc.Proc{
			{PID: 10, PPID: 1}, {PID: 11, PPID: 10}, {PID: 12, PPID: 11}}, []int{10}},
		{"two roots (helper parented outside)", []proc.Proc{
			{PID: 20, PPID: 1}, {PID: 21, PPID: 20}, {PID: 22, PPID: 1}}, []int{20, 22}},
		{"unsorted input yields sorted roots", []proc.Proc{
			{PID: 33, PPID: 1}, {PID: 31, PPID: 1}, {PID: 32, PPID: 31}}, []int{31, 33}},
		{"ppid cycle falls back to all pids", []proc.Proc{
			{PID: 40, PPID: 41}, {PID: 41, PPID: 40}}, []int{40, 41}},
	}
	for _, tt := range tests {
		got := rootPIDs(Group{Procs: tt.procs})
		if len(got) != len(tt.want) {
			t.Errorf("%s: rootPIDs = %v, want %v", tt.name, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("%s: rootPIDs = %v, want %v", tt.name, got, tt.want)
				break
			}
		}
	}
}
