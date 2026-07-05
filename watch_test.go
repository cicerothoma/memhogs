package main

import (
	"testing"

	"github.com/cicerothoma/memhogs/internal/group"
	"github.com/cicerothoma/memhogs/internal/proc"
	"github.com/cicerothoma/memhogs/internal/render"
)

func groupWith(n int) group.Group {
	g := group.Group{}
	for i := 0; i < n; i++ {
		g.Procs = append(g.Procs, proc.Proc{PID: i + 1})
	}
	return g
}

func TestGroupRows(t *testing.T) {
	tree := render.Opts{Tree: true, MaxMembers: 5}
	cases := []struct {
		name  string
		procs int
		opts  render.Opts
		want  int
	}{
		{"single process is one row", 1, tree, 1},
		{"members expand under the group row", 3, tree, 4},
		{"capped members add a fold row", 8, tree, 7}, // 1 + 5 members + "… 3 more"
		{"at the cap there is no fold row", 5, tree, 6},
		{"compact view is always one row", 8, render.Opts{}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := groupRows(groupWith(tc.procs), tc.opts); got != tc.want {
				t.Errorf("groupRows = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestFitTop(t *testing.T) {
	opts := render.Opts{Tree: true, MaxMembers: 5}
	groups := []group.Group{groupWith(3), groupWith(1), groupWith(8)} // 4 + 1 + 7 rows
	cases := []struct {
		name   string
		budget int
		want   int
	}{
		{"everything fits", 20, 3},
		{"third group does not fit", 11, 2},
		{"only the first fits", 4, 1},
		{"always at least one group", 0, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := fitTop(groups, opts, tc.budget); got != tc.want {
				t.Errorf("fitTop(budget=%d) = %d, want %d", tc.budget, got, tc.want)
			}
		})
	}
}

func TestClipLine(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		width int
		want  string
	}{
		{"short line untouched", "hello", 10, "hello"},
		{"plain cut", "hello world", 5, "hello"},
		{"escapes cost no columns", "\x1b[33mhello\x1b[0m world", 5, "\x1b[33mhello\x1b[0m"},
		{"cut inside color keeps the reset", "\x1b[36mhello world\x1b[0m", 5, "\x1b[36mhello\x1b[0m"},
		{"exact width untouched", "hello", 5, "hello"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := clipLine(tc.in, tc.width); got != tc.want {
				t.Errorf("clipLine(%q, %d) = %q, want %q", tc.in, tc.width, got, tc.want)
			}
		})
	}
}

func TestLowerTop(t *testing.T) {
	if got := lowerTop(0, 7); got != 7 {
		t.Errorf("unlimited top should adopt the fit, got %d", got)
	}
	if got := lowerTop(3, 7); got != 3 {
		t.Errorf("a user top below the fit should win, got %d", got)
	}
	if got := lowerTop(10, 7); got != 7 {
		t.Errorf("the fit should tighten a larger user top, got %d", got)
	}
}
