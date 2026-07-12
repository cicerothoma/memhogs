// memhogs lists applications and services by memory use, rolling helper and
// child processes up into the app that owns them.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/cicerothoma/memhogs/internal/group"
	"github.com/cicerothoma/memhogs/internal/proc"
	"github.com/cicerothoma/memhogs/internal/render"
)

// version is stamped by the release build (goreleaser -X main.version=...).
var version = "dev"

func main() {
	// `memhogs stop <name>` (alias `kill`) is a subcommand, not a flag: it prints
	// how to stop the matching group(s) and never runs anything itself.
	if len(os.Args) > 1 && (os.Args[1] == "stop" || os.Args[1] == "kill") {
		stopCommand(os.Args[2:])
		return
	}

	var (
		showVersion = flag.Bool("version", false, "print version and exit")
		top         = flag.Int("top", 0, "show only the top N entries")
		flat        = flag.Bool("flat", false, "list individual processes instead of grouping by app")
		jsonOut     = flag.Bool("json", false, "emit JSON")
		tree        = flag.Bool("tree", false, "expand every member process (default shows top 5 per group)")
		compact     = flag.Bool("compact", false, "one row per group, no member processes")
		watch       = flag.Bool("watch", false, "full-screen live view that refreshes in place")
		interval    = flag.Duration("interval", 2*time.Second, "refresh interval for --watch")
		rss         = flag.Bool("rss", false, "use RSS (ps/top-comparable) instead of the fair-share metric")
		stopHint    = flag.Bool("stop-hint", false, "under each group, show the command to stop it")
		noColor     = flag.Bool("no-color", false, "disable colored output")
	)
	flag.Usage = usage
	filter := strings.ToLower(strings.Join(parseFlagsAnywhere(flag.CommandLine, os.Args[1:]), " "))

	if *showVersion {
		fmt.Println("memhogs " + version)
		return
	}

	if *flat && *tree {
		fatal("--flat and --tree are mutually exclusive")
	}
	if *tree && *compact {
		fatal("--tree and --compact are mutually exclusive")
	}
	if *watch && *jsonOut {
		fatal("--watch and --json are mutually exclusive")
	}
	if *stopHint && *flat {
		fatal("--stop-hint applies to grouped views, not --flat (a flat row is just kill <pid>)")
	}
	if *stopHint && *watch {
		fatal("--stop-hint and --watch are mutually exclusive")
	}

	hooks := platformHooks()
	// Default view: tree capped at 5 members per group. --tree lifts the
	// cap, --compact collapses to one row per group.
	opts := render.Opts{Top: *top, Tree: !*compact, TotalMem: proc.TotalMemory()}
	if !*tree {
		opts.MaxMembers = 5
	}
	opts.Color = !*noColor && !*jsonOut && os.Getenv("NO_COLOR") == "" && stdoutIsTTY()
	if *stopHint {
		opts.StopCmd = func(g group.Group) string { return group.StopRecipe(g, runtime.GOOS).Stop }
	}

	if *watch {
		watchLoop(hooks, opts, filter, *flat, *rss, *interval)
		return
	}
	if err := run(os.Stdout, hooks, opts, filter, *flat, *jsonOut, *rss, 0); err != nil {
		fatal(err.Error())
	}
}

// run takes one snapshot and renders it to w. rowBudget > 0 caps the output
// at that many lines by lowering the effective --top; 0 means unlimited.
func run(w io.Writer, hooks group.Hooks, opts render.Opts, filter string, flat, jsonOut, rss bool, rowBudget int) error {
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
		if fit := rowBudget - tableChrome; rowBudget > 0 {
			opts.Top = lowerTop(opts.Top, max(fit, 1))
		}
		if jsonOut {
			return render.JSONFlat(w, procs, opts)
		}
		render.FlatTable(w, procs, opts)
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
	if rowBudget > 0 {
		opts.Top = lowerTop(opts.Top, fitTop(groups, opts, rowBudget-tableChrome))
	}
	if jsonOut {
		return render.JSON(w, groups, opts)
	}
	render.Table(w, groups, opts)
	return nil
}

// tableChrome is the fixed rows around the table body: the column header
// plus the blank line and two-line summary footer.
const tableChrome = 4

// lowerTop tightens a --top value (0 = unlimited) to at most fit.
func lowerTop(top, fit int) int {
	if top == 0 || fit < top {
		return fit
	}
	return top
}

// parseFlagsAnywhere parses flags wherever they appear on the command line
// and returns the positional arguments, so `memhogs chrome --flat` means the
// same as `memhogs --flat chrome`. The flag package alone stops at the first
// positional argument and would silently fold `--flat` into the filter. A
// literal "--" still marks everything after it as positional.
func parseFlagsAnywhere(fs *flag.FlagSet, args []string) []string {
	var tail []string
	for i, a := range args {
		if a == "--" {
			tail = args[i+1:]
			args = args[:i]
			break
		}
	}
	var positionals []string
	for len(args) > 0 {
		fs.Parse(args)
		rest := fs.Args()
		i := 0
		for i < len(rest) && (rest[i] == "-" || !strings.HasPrefix(rest[i], "-")) {
			i++
		}
		positionals = append(positionals, rest[:i]...)
		if i == len(rest) {
			break
		}
		args = rest[i:]
	}
	return append(positionals, tail...)
}

// stopCommand implements `memhogs stop <name>` (alias `kill`): it finds the
// groups whose name matches and prints the command(s) to stop each. It is
// deliberately read-only — memhogs shows you the command; you decide whether
// to run it.
func stopCommand(args []string) {
	filter := strings.ToLower(strings.Join(args, " "))
	if filter == "" {
		fatal("usage: memhogs stop <name>   (name of the app/service to stop)")
	}
	snapshot, err := proc.Snapshot()
	if err != nil {
		fatal("reading processes: " + err.Error())
	}
	groups := group.Build(snapshot, platformHooks(), proc.MemFair)
	var matched []group.Group
	for _, g := range groups {
		if matches(filter, g.Name) {
			matched = append(matched, g)
		}
	}
	if len(matched) == 0 {
		fatal(fmt.Sprintf("no group matches %q — run `memhogs %s` to see the names", filter, filter))
	}
	for _, g := range matched {
		r := group.StopRecipe(g, runtime.GOOS)
		fmt.Printf("%s — %d process(es), %s\n", g.Name, len(g.Procs), render.HumanBytes(g.Mem))
		fmt.Printf("  stop:  %s\n", r.Stop)
		if r.Force != r.Stop {
			fmt.Printf("  force: %s\n", r.Force)
		}
		fmt.Println()
	}
	fmt.Println("memhogs only prints these — copy the one you want to run it.")
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

func stdoutIsTTY() bool {
	fi, err := os.Stdout.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
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
       memhogs stop <name>     show how to stop a matching app/service

  filter          only show groups whose name contains this substring
                  (flags may come before or after it)

The stop subcommand (alias: kill) and --stop-hint only ever print a stop command
(systemctl stop / osascript quit / kill); memhogs never terminates anything.

flags:
`)
	flag.PrintDefaults()
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "memhogs: "+msg)
	os.Exit(1)
}
