package render

import (
	"strings"
	"testing"

	"memhogs/internal/group"
	"memhogs/internal/proc"
)

func TestHumanBytes(t *testing.T) {
	tests := []struct {
		in   uint64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{10 * 1024, "10.0 KiB"},
		{117 * 1024 * 1024, "117.0 MiB"},
		{7278 * 1024 * 1024, "7.1 GiB"},
	}
	for _, tt := range tests {
		if got := HumanBytes(tt.in); got != tt.want {
			t.Errorf("HumanBytes(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTableTopAndFooter(t *testing.T) {
	groups := []group.Group{
		{Name: "Dia", Kind: group.KindApp, RSS: 7 << 30, Procs: []proc.Proc{{PID: 1, RSS: 7 << 30, Name: "Dia"}}},
		{Name: "Cursor", Kind: group.KindApp, RSS: 2 << 30, Procs: []proc.Proc{{PID: 2, RSS: 2 << 30, Name: "Cursor"}}},
		{Name: "Slack", Kind: group.KindApp, RSS: 1 << 30, Procs: []proc.Proc{{PID: 3, RSS: 1 << 30, Name: "Slack"}}},
	}
	var b strings.Builder
	Table(&b, groups, Opts{Top: 2, TotalMem: 64 << 30})
	out := b.String()
	if strings.Contains(out, "Slack") {
		t.Error("Top=2 should truncate Slack")
	}
	if !strings.Contains(out, "2 of 3 groups") || !strings.Contains(out, "10.0 GiB") {
		t.Errorf("footer should count all groups and total RSS:\n%s", out)
	}
	if !strings.Contains(out, "PROCESSES") {
		t.Errorf("header should spell out PROCESSES:\n%s", out)
	}
	if !strings.Contains(out, "10.9%") { // 7 GiB of 64 GiB
		t.Errorf("expected %%MEM column with 10.9%% for Dia:\n%s", out)
	}
	if !strings.Contains(out, "of 64.0 GiB RAM") {
		t.Errorf("footer should mention total RAM:\n%s", out)
	}
}

func TestTableWithoutTotalMemOmitsPercent(t *testing.T) {
	groups := []group.Group{
		{Name: "Dia", Kind: group.KindApp, RSS: 7 << 30, Procs: []proc.Proc{{PID: 1, RSS: 7 << 30, Name: "Dia"}}},
	}
	var b strings.Builder
	Table(&b, groups, Opts{})
	if strings.Contains(b.String(), "%") && strings.Contains(b.String(), "10.9") {
		t.Errorf("no percent values expected without TotalMem:\n%s", b.String())
	}
}

func TestTreeListsMembers(t *testing.T) {
	groups := []group.Group{
		{Name: "Cursor", Kind: group.KindApp, RSS: 3 << 20, Procs: []proc.Proc{
			{PID: 301, RSS: 2 << 20, Name: "Cursor Helper (Plugin)"},
			{PID: 302, RSS: 1 << 20, Name: "tsserver"},
		}},
	}
	var b strings.Builder
	Table(&b, groups, Opts{Tree: true})
	out := b.String()
	if !strings.Contains(out, "├─") || !strings.Contains(out, "└─") || !strings.Contains(out, "tsserver [302]") {
		t.Errorf("tree output missing members:\n%s", out)
	}
}
