// Package render prints grouped or flat snapshots as a table or JSON.
package render

import (
	"encoding/json"
	"fmt"
	"io"

	"memhogs/internal/group"
	"memhogs/internal/proc"
)

type Opts struct {
	Top      int    // show only the first N groups/processes; 0 = all
	Tree     bool   // list member processes under each group
	TotalMem uint64 // physical RAM in bytes; enables the %MEM column when > 0
}

func (o Opts) pct(rss uint64) string {
	if o.TotalMem == 0 {
		return ""
	}
	return fmt.Sprintf("%.1f%%", float64(rss)/float64(o.TotalMem)*100)
}

func Table(w io.Writer, groups []group.Group, o Opts) {
	shown := groups
	if o.Top > 0 && o.Top < len(shown) {
		shown = shown[:o.Top]
	}
	fmt.Fprintf(w, "%10s  %6s  %9s  %s\n", "MEMORY", "%MEM", "PROCESSES", "NAME")
	for _, g := range shown {
		fmt.Fprintf(w, "%10s  %6s  %9d  %s\n", HumanBytes(g.RSS), o.pct(g.RSS), len(g.Procs), g.Name)
		if o.Tree {
			for i, p := range g.Procs {
				branch := "├─"
				if i == len(g.Procs)-1 {
					branch = "└─"
				}
				fmt.Fprintf(w, "%10s  %6s  %9s  %s %s  %s [%d]\n", "", "", "", branch, HumanBytes(p.RSS), p.Name, p.PID)
			}
		}
	}
	var total uint64
	nprocs := 0
	for _, g := range groups {
		total += g.RSS
		nprocs += len(g.Procs)
	}
	fmt.Fprintf(w, "\n%d of %d groups · %d processes · total %s%s\n",
		len(shown), len(groups), nprocs, HumanBytes(total), footerSuffix(o))
}

func FlatTable(w io.Writer, procs []proc.Proc, o Opts) {
	shown := procs
	if o.Top > 0 && o.Top < len(shown) {
		shown = shown[:o.Top]
	}
	fmt.Fprintf(w, "%10s  %6s  %7s  %s\n", "MEMORY", "%MEM", "PID", "NAME")
	for _, p := range shown {
		fmt.Fprintf(w, "%10s  %6s  %7d  %s\n", HumanBytes(p.RSS), o.pct(p.RSS), p.PID, p.Name)
	}
	var total uint64
	for _, p := range procs {
		total += p.RSS
	}
	fmt.Fprintf(w, "\n%d of %d processes · total %s%s\n",
		len(shown), len(procs), HumanBytes(total), footerSuffix(o))
}

func footerSuffix(o Opts) string {
	if o.TotalMem == 0 {
		return " (RSS; shared memory counted per process)"
	}
	return fmt.Sprintf(" of %s RAM (RSS; shared memory counted per process)", HumanBytes(o.TotalMem))
}

type jsonProc struct {
	PID      int    `json:"pid"`
	PPID     int    `json:"ppid"`
	RSSBytes uint64 `json:"rss_bytes"`
	Name     string `json:"name"`
	Path     string `json:"path,omitempty"`
}

type jsonGroup struct {
	Name       string     `json:"name"`
	Kind       string     `json:"kind"`
	RSSBytes   uint64     `json:"rss_bytes"`
	RSS        string     `json:"rss"`
	PercentRAM float64    `json:"percent_of_ram,omitempty"`
	Procs      []jsonProc `json:"procs"`
}

func JSON(w io.Writer, groups []group.Group, o Opts) error {
	if o.Top > 0 && o.Top < len(groups) {
		groups = groups[:o.Top]
	}
	out := struct {
		TotalRAMBytes uint64      `json:"total_ram_bytes,omitempty"`
		Groups        []jsonGroup `json:"groups"`
	}{TotalRAMBytes: o.TotalMem, Groups: make([]jsonGroup, len(groups))}
	for i, g := range groups {
		jg := jsonGroup{Name: g.Name, Kind: g.Kind.String(), RSSBytes: g.RSS, RSS: HumanBytes(g.RSS), Procs: make([]jsonProc, len(g.Procs))}
		if o.TotalMem > 0 {
			jg.PercentRAM = roundPct(float64(g.RSS) / float64(o.TotalMem) * 100)
		}
		for j, p := range g.Procs {
			jg.Procs[j] = jsonProc{PID: p.PID, PPID: p.PPID, RSSBytes: p.RSS, Name: p.Name, Path: p.Path}
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
		TotalRAMBytes uint64     `json:"total_ram_bytes,omitempty"`
		Procs         []jsonProc `json:"procs"`
	}{TotalRAMBytes: o.TotalMem, Procs: make([]jsonProc, len(procs))}
	for i, p := range procs {
		out.Procs[i] = jsonProc{PID: p.PID, PPID: p.PPID, RSSBytes: p.RSS, Name: p.Name, Path: p.Path}
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
