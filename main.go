// memhogs lists applications and services by memory use, rolling helper and
// child processes up into the app that owns them.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"memhogs/internal/group"
	"memhogs/internal/proc"
	"memhogs/internal/render"
)

func main() {
	var (
		top      = flag.Int("top", 0, "show only the top N entries")
		flat     = flag.Bool("flat", false, "list individual processes instead of grouping by app")
		jsonOut  = flag.Bool("json", false, "emit JSON")
		tree     = flag.Bool("tree", false, "expand every member process (default shows top 5 per group)")
		compact  = flag.Bool("compact", false, "one row per group, no member processes")
		watch    = flag.Bool("watch", false, "refresh continuously")
		interval = flag.Duration("interval", 2*time.Second, "refresh interval for --watch")
		rss      = flag.Bool("rss", false, "use RSS (ps/top-comparable) instead of the fair-share metric")
	)
	flag.Usage = usage
	flag.Parse()

	filter := strings.ToLower(strings.Join(flag.Args(), " "))
	if *flat && *tree {
		fatal("--flat and --tree are mutually exclusive")
	}
	if *tree && *compact {
		fatal("--tree and --compact are mutually exclusive")
	}
	if *watch && *jsonOut {
		fatal("--watch and --json are mutually exclusive")
	}

	hooks := platformHooks()
	// Default view: tree capped at 5 members per group. --tree lifts the
	// cap, --compact collapses to one row per group.
	opts := render.Opts{Top: *top, Tree: !*compact, TotalMem: proc.TotalMemory()}
	if !*tree {
		opts.MaxMembers = 5
	}

	for {
		if *watch {
			fmt.Print("\x1b[H\x1b[2J") // clear screen, cursor home
			fmt.Printf("memhogs · %s · every %s (ctrl-c to quit)\n\n", time.Now().Format("15:04:05"), *interval)
		}
		if err := run(hooks, opts, filter, *flat, *jsonOut, *rss); err != nil {
			fatal(err.Error())
		}
		if !*watch {
			return
		}
		time.Sleep(*interval)
	}
}

func run(hooks group.Hooks, opts render.Opts, filter string, flat, jsonOut, rss bool) error {
	snapshot, err := proc.Snapshot()
	if err != nil {
		return fmt.Errorf("reading processes: %w", err)
	}
	memOf := proc.MemFair
	if rss {
		memOf = proc.MemRSS
	}
	opts.MemOf = memOf
	opts.Metric, opts.Fallback = metricInfo(snapshot, rss)

	if flat {
		procs := snapshot[:0:0]
		for _, p := range snapshot {
			if memOf(p) == 0 || !matches(filter, p.Name, p.Path) {
				continue
			}
			procs = append(procs, p)
		}
		sort.Slice(procs, func(i, j int) bool { return memOf(procs[i]) > memOf(procs[j]) })
		if jsonOut {
			return render.JSONFlat(os.Stdout, procs, opts)
		}
		render.FlatTable(os.Stdout, procs, opts)
		return nil
	}

	groups := group.Build(snapshot, hooks, memOf)
	if filter != "" {
		kept := groups[:0:0]
		for _, g := range groups {
			if matches(filter, g.Name) {
				kept = append(kept, g)
			}
		}
		groups = kept
	}
	if jsonOut {
		return render.JSON(os.Stdout, groups, opts)
	}
	render.Table(os.Stdout, groups, opts)
	return nil
}

func matches(filter string, fields ...string) bool {
	if filter == "" {
		return true
	}
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f), filter) {
			return true
		}
	}
	return false
}

// metricInfo names the effective metric and counts how many live processes
// fell back to RSS. If none could be read fairly (no cgo, old kernel), the
// metric is effectively RSS and is reported as such.
func metricInfo(snapshot []proc.Proc, rss bool) (string, int) {
	if rss {
		return "rss", 0
	}
	fallback, alive := 0, 0
	for _, p := range snapshot {
		if p.RSS == 0 {
			continue
		}
		alive++
		if p.Fair == 0 {
			fallback++
		}
	}
	if alive > 0 && fallback == alive {
		return "rss", 0
	}
	if runtime.GOOS == "linux" {
		return "pss", fallback
	}
	return "footprint", fallback
}

func platformHooks() group.Hooks {
	if runtime.GOOS == "linux" {
		return group.LinuxHooks()
	}
	return group.DarwinHooks()
}

func usage() {
	fmt.Fprintf(os.Stderr, `memhogs — apps and services by memory use, largest first

Helper and child processes roll up into the app that owns them (via the
process tree), so Electron helpers, language servers, etc. count toward
their parent app. Memory is a fair-share metric by default (footprint on
macOS — the Activity Monitor number — and PSS on Linux), so shared pages
are not double-counted; pass --rss for ps/top-comparable numbers.

By default each group expands to its 5 biggest member processes; --tree
shows all of them, --compact shows one row per group.

usage: memhogs [flags] [filter]

  filter          only show groups whose name contains this substring

flags:
`)
	flag.PrintDefaults()
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "memhogs: "+msg)
	os.Exit(1)
}
