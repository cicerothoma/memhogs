package render

import (
	"strings"
	"testing"

	"github.com/cicerothoma/memhogs/internal/group"
	"github.com/cicerothoma/memhogs/internal/proc"
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
		{Name: "Dia", Kind: group.KindApp, Mem: 7 << 30, Procs: []proc.Proc{{PID: 1, RSS: 7 << 30, Name: "Dia"}}},
		{Name: "Cursor", Kind: group.KindApp, Mem: 2 << 30, Procs: []proc.Proc{{PID: 2, RSS: 2 << 30, Name: "Cursor"}}},
		{Name: "Slack", Kind: group.KindApp, Mem: 1 << 30, Procs: []proc.Proc{{PID: 3, RSS: 1 << 30, Name: "Slack"}}},
	}
	var b strings.Builder
	Table(&b, groups, Opts{Top: 2, TotalMem: 64 << 30, Metric: "footprint", Fallback: 12})
	out := b.String()
	if strings.Contains(out, "Slack") {
		t.Error("Top=2 should truncate Slack")
	}
	if !strings.Contains(out, "2 of 3 groups") || !strings.Contains(out, "10.0 GiB") {
		t.Errorf("footer should count all groups and total mem:\n%s", out)
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
	if !strings.Contains(out, "Activity Monitor") || !strings.Contains(out, "12 unreadable") {
		t.Errorf("footer should name the footprint metric and fallback count:\n%s", out)
	}
}

func TestRSSMetricFooter(t *testing.T) {
	groups := []group.Group{
		{Name: "Dia", Kind: group.KindApp, Mem: 7 << 30, Procs: []proc.Proc{{PID: 1, RSS: 7 << 30, Name: "Dia"}}},
	}
	var b strings.Builder
	Table(&b, groups, Opts{Metric: "rss"})
	if !strings.Contains(b.String(), "metric: RSS") {
		t.Errorf("footer should flag the RSS metric and its caveat:\n%s", b.String())
	}
}

func TestTreeUsesSelectedMetric(t *testing.T) {
	groups := []group.Group{
		{Name: "Cursor", Kind: group.KindApp, Mem: 3 << 20, Procs: []proc.Proc{
			{PID: 301, RSS: 200 << 20, Fair: 2 << 20, Name: "Cursor Helper (Plugin)"},
			{PID: 302, RSS: 100 << 20, Fair: 1 << 20, Name: "tsserver"},
		}},
	}
	var b strings.Builder
	Table(&b, groups, Opts{Tree: true, MemOf: proc.MemFair})
	out := b.String()
	if !strings.Contains(out, "├─") || !strings.Contains(out, "└─") || !strings.Contains(out, "tsserver [302]") {
		t.Errorf("tree output missing members:\n%s", out)
	}
	if !strings.Contains(out, "2.0 MiB") || strings.Contains(out, "200.0 MiB") {
		t.Errorf("tree rows should show the fair metric, not RSS:\n%s", out)
	}
}

func TestTreeCapFoldsRemainder(t *testing.T) {
	procs := make([]proc.Proc, 8)
	var total uint64
	for i := range procs {
		mem := uint64(8-i) << 20
		procs[i] = proc.Proc{PID: 100 + i, RSS: mem, Name: "helper"}
		total += mem
	}
	groups := []group.Group{
		{Name: "Dia", Kind: group.KindApp, Mem: total, Procs: procs},
		{Name: "solo", Kind: group.KindStandalone, Mem: 1 << 20, Procs: []proc.Proc{{PID: 1, RSS: 1 << 20, Name: "solo"}}},
	}
	var b strings.Builder
	Table(&b, groups, Opts{Tree: true, MaxMembers: 5})
	out := b.String()
	if !strings.Contains(out, "… 3 more (6.0 MiB)") { // members 6,7,8 = 3+2+1 MiB
		t.Errorf("expected folded remainder line:\n%s", out)
	}
	if strings.Count(out, "├─") != 5 {
		t.Errorf("expected exactly 5 expanded members:\n%s", out)
	}
	if strings.Contains(out, "solo [1]") {
		t.Errorf("single-process group must not expand:\n%s", out)
	}
}
