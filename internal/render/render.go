// Package render prints grouped or flat snapshots as a table or JSON.
package render

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/cicerothoma/memhogs/internal/group"
	"github.com/cicerothoma/memhogs/internal/proc"
)

type Opts struct {
	Top        int                      // show only the first N groups/processes; 0 = all
	Tree       bool                     // list member processes under multi-process groups
	MaxMembers int                      // member rows per group before folding into "… N more"; 0 = all
	TotalMem   uint64                   // physical RAM in bytes; enables the %MEM column when > 0
	MemOf      func(proc.Proc) uint64   // per-process metric selector; nil = RSS
	Metric     string                   // metric name for the footer: "footprint", "pss", or "rss"
	Fallback   int                      // processes counted via RSS because the metric was unreadable
	Color      bool                     // wrap output in ANSI colors
	StopCmd    func(group.Group) string // if set, print/emit a stop command per group
}

// ANSI SGR codes. Strings are padded to column width before wrapping so
// escape sequences never skew alignment.
const (
	cDim   = "2"
	cMem   = "33"   // amber: memory values
	cApp   = "36"   // cyan: recognized applications/services
	cAlone = "32"   // green: standalone groups
	cHot   = "1;31" // bold red: %MEM worth worrying about
)

const hotShare = 0.15 // fraction of RAM at which %MEM turns red

func (o Opts) c(code, s string) string {
	if !o.Color {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func (o Opts) mem(p proc.Proc) uint64 {
	if o.MemOf == nil {
		return p.RSS
	}
	return o.MemOf(p)
}

func (o Opts) pct(mem uint64) string {
	if o.TotalMem == 0 {
		return ""
	}
	return fmt.Sprintf("%.1f%%", float64(mem)/float64(o.TotalMem)*100)
}

func Table(w io.Writer, groups []group.Group, o Opts) {
	shown := groups
	if o.Top > 0 && o.Top < len(shown) {
		shown = shown[:o.Top]
	}
	fmt.Fprintln(w, o.c(cDim, fmt.Sprintf("%10s  %6s  %9s  %s", "MEMORY", "%MEM", "PROCESSES", "NAME")))
	indent := "                               " // width of the three number columns
	for _, g := range shown {
		nameCode := cApp
		if g.Kind == group.KindStandalone {
			nameCode = cAlone
		}
		fmt.Fprintf(w, "%s  %s  %9d  %s\n",
			o.c(cMem, fmt.Sprintf("%10s", HumanBytes(g.Mem))), o.hotPct(g.Mem), len(g.Procs), o.c(nameCode, g.Name))
		// A single member would just repeat the group row, so only expand real groups.
		if o.Tree && len(g.Procs) > 1 {
			members := g.Procs
			if o.MaxMembers > 0 && o.MaxMembers < len(members) {
				members = members[:o.MaxMembers]
			}
			for i, p := range members {
				branch := "├─"
				if i == len(members)-1 && len(members) == len(g.Procs) {
					branch = "└─"
				}
				fmt.Fprintf(w, "%s%s %s  %s %s\n",
					indent, o.c(cDim, branch), o.c(cMem, fmt.Sprintf("%9s", HumanBytes(o.mem(p)))), p.Name, o.c(cDim, fmt.Sprintf("[%d]", p.PID)))
			}
			if rest := g.Procs[len(members):]; len(rest) > 0 {
				var restMem uint64
				for _, p := range rest {
					restMem += o.mem(p)
				}
				fmt.Fprintf(w, "%s%s\n", indent, o.c(cDim, fmt.Sprintf("└─ … %d more (%s)", len(rest), HumanBytes(restMem))))
			}
		}
		if o.StopCmd != nil {
			if cmd := o.StopCmd(g); cmd != "" {
				fmt.Fprintf(w, "%s%s\n", indent, o.c(cDim, "stop ▸ "+cmd))
			}
		}
	}
	var total uint64
	nprocs := 0
	for _, g := range groups {
		total += g.Mem
		nprocs += len(g.Procs)
	}
	fmt.Fprintf(w, "\n%s\n%s\n",
		o.c(cDim, fmt.Sprintf("%d of %d groups · %d processes · total %s%s", len(shown), len(groups), nprocs, HumanBytes(total), ramSuffix(o))),
		o.c(cDim, metricLine(o)))
}

// hotPct renders the %MEM cell, flagging shares above hotShare in red.
func (o Opts) hotPct(mem uint64) string {
	s := fmt.Sprintf("%6s", o.pct(mem))
	if o.TotalMem > 0 && float64(mem)/float64(o.TotalMem) >= hotShare {
		return o.c(cHot, s)
	}
	return s
}

func FlatTable(w io.Writer, procs []proc.Proc, o Opts) {
	shown := procs
	if o.Top > 0 && o.Top < len(shown) {
		shown = shown[:o.Top]
	}
	fmt.Fprintln(w, o.c(cDim, fmt.Sprintf("%10s  %6s  %7s  %s", "MEMORY", "%MEM", "PID", "NAME")))
	for _, p := range shown {
		fmt.Fprintf(w, "%s  %s  %7d  %s\n",
			o.c(cMem, fmt.Sprintf("%10s", HumanBytes(o.mem(p)))), o.hotPct(o.mem(p)), p.PID, o.c(cApp, p.Name))
	}
	var total uint64
	for _, p := range procs {
		total += o.mem(p)
	}
	fmt.Fprintf(w, "\n%s\n%s\n",
		o.c(cDim, fmt.Sprintf("%d of %d processes · total %s%s", len(shown), len(procs), HumanBytes(total), ramSuffix(o))),
		o.c(cDim, metricLine(o)))
}

func ramSuffix(o Opts) string {
	if o.TotalMem == 0 {
		return ""
	}
	return fmt.Sprintf(" of %s RAM", HumanBytes(o.TotalMem))
}

func metricLine(o Opts) string {
	switch o.Metric {
	case "footprint":
		s := "metric: memory footprint (same as Activity Monitor)"
		if o.Fallback > 0 {
			s += fmt.Sprintf("; %d unreadable processes counted via RSS", o.Fallback)
		}
		return s
	case "pss":
		s := "metric: PSS (shared pages charged fractionally)"
		if o.Fallback > 0 {
			s += fmt.Sprintf("; %d unreadable processes counted via RSS", o.Fallback)
		}
		return s
	default:
		return "metric: RSS (shared memory counted once per process, so totals overstate)"
	}
}

type jsonProc struct {
	PID      int    `json:"pid"`
	PPID     int    `json:"ppid"`
	MemBytes uint64 `json:"mem_bytes"`
	RSSBytes uint64 `json:"rss_bytes"`
	Name     string `json:"name"`
	Path     string `json:"path,omitempty"`
}

type jsonGroup struct {
	Name       string     `json:"name"`
	Kind       string     `json:"kind"`
	MemBytes   uint64     `json:"mem_bytes"`
	Mem        string     `json:"mem"`
	PercentRAM float64    `json:"percent_of_ram,omitempty"`
	Stop       string     `json:"stop,omitempty"`
	Procs      []jsonProc `json:"procs"`
}

func (o Opts) jsonProc(p proc.Proc) jsonProc {
	return jsonProc{PID: p.PID, PPID: p.PPID, MemBytes: o.mem(p), RSSBytes: p.RSS, Name: p.Name, Path: p.Path}
}

func JSON(w io.Writer, groups []group.Group, o Opts) error {
	if o.Top > 0 && o.Top < len(groups) {
		groups = groups[:o.Top]
	}
	out := struct {
		Metric        string      `json:"metric"`
		TotalRAMBytes uint64      `json:"total_ram_bytes,omitempty"`
		Groups        []jsonGroup `json:"groups"`
	}{Metric: o.Metric, TotalRAMBytes: o.TotalMem, Groups: make([]jsonGroup, len(groups))}
	for i, g := range groups {
		jg := jsonGroup{Name: g.Name, Kind: g.Kind.String(), MemBytes: g.Mem, Mem: HumanBytes(g.Mem), Procs: make([]jsonProc, len(g.Procs))}
		if o.TotalMem > 0 {
			jg.PercentRAM = roundPct(float64(g.Mem) / float64(o.TotalMem) * 100)
		}
		if o.StopCmd != nil {
			jg.Stop = o.StopCmd(g)
		}
		for j, p := range g.Procs {
			jg.Procs[j] = o.jsonProc(p)
		}
		out.Groups[i] = jg
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func JSONFlat(w io.Writer, procs []proc.Proc, o Opts) error {
	if o.Top > 0 && o.Top < len(procs) {
		procs = procs[:o.Top]
	}
	out := struct {
		Metric        string     `json:"metric"`
		TotalRAMBytes uint64     `json:"total_ram_bytes,omitempty"`
		Procs         []jsonProc `json:"procs"`
	}{Metric: o.Metric, TotalRAMBytes: o.TotalMem, Procs: make([]jsonProc, len(procs))}
	for i, p := range procs {
		out.Procs[i] = o.jsonProc(p)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func roundPct(f float64) float64 {
	return float64(int(f*10+0.5)) / 10
}

// HumanBytes formats a byte count in binary units with one decimal.
func HumanBytes(b uint64) string {
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(1024), 0
	for n := b / 1024; n >= 1024; n /= 1024 {
		div *= 1024
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
