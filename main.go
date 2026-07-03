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
		tree     = flag.Bool("tree", false, "list member processes under each group")
		watch    = flag.Bool("watch", false, "refresh continuously")
		interval = flag.Duration("interval", 2*time.Second, "refresh interval for --watch")
	)
	flag.Usage = usage
	flag.Parse()

	filter := strings.ToLower(strings.Join(flag.Args(), " "))
	if *flat && *tree {
		fatal("--flat and --tree are mutually exclusive")
	}
	if *watch && *jsonOut {
		fatal("--watch and --json are mutually exclusive")
	}

	hooks := platformHooks()
	opts := render.Opts{Top: *top, Tree: *tree, TotalMem: proc.TotalMemory()}

	for {
		if *watch {
			fmt.Print("\x1b[H\x1b[2J") // clear screen, cursor home
			fmt.Printf("memhogs · %s · every %s (ctrl-c to quit)\n\n", time.Now().Format("15:04:05"), *interval)
		}
		if err := run(hooks, opts, filter, *flat, *jsonOut); err != nil {
			fatal(err.Error())
		}
		if !*watch {
			return
		}
		time.Sleep(*interval)
	}
}

func run(hooks group.Hooks, opts render.Opts, filter string, flat, jsonOut bool) error {
	snapshot, err := proc.Snapshot()
	if err != nil {
		return fmt.Errorf("reading processes: %w", err)
	}

	if flat {
		procs := snapshot[:0:0]
		for _, p := range snapshot {
			if p.RSS == 0 || !matches(filter, p.Name, p.Path) {
				continue
			}
			procs = append(procs, p)
		}
		sort.Slice(procs, func(i, j int) bool { return procs[i].RSS > procs[j].RSS })
		if jsonOut {
			return render.JSONFlat(os.Stdout, procs, opts)
		}
		render.FlatTable(os.Stdout, procs, opts)
		return nil
	}

	groups := group.Build(snapshot, hooks)
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
their parent app. Memory is RSS: shared pages are counted once per process,
so grouped totals can overstate real usage somewhat.

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
