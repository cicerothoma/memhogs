# memhogs

A command line tool that shows which applications are using your memory.
It groups helper and child processes under the app that owns them, so an
Electron app with 40 renderer processes shows up as one entry with one
total, not 40 scattered rows.

![memhogs demo: the default tree view, the compact view, and filtering by name](docs/demo.gif)

```
$ memhogs --top 3
    MEMORY    %MEM  PROCESSES  NAME
   7.3 GiB   30.4%         46  Dia
                               ├─ 870.2 MiB  Dia [12251]
                               ├─ 540.7 MiB  Browser Helper [12257]
                               ├─ 481.2 MiB  Browser Helper (Renderer) [16103]
                               ├─ 430.7 MiB  Browser Helper [12264]
                               ├─ 360.0 MiB  Browser Helper (Renderer) [12604]
                               └─ … 41 more (4.7 GiB)
   2.1 GiB    8.8%         23  Visual Studio Code
                               ├─ 464.0 MiB  Code Helper (Plugin) [10609]
                               └─ … 22 more (1.6 GiB)
 708.8 MiB    2.9%          6  Slack
                               ├─ 368.4 MiB  Slack Helper (Renderer) [17182]
                               └─ … 5 more (340.4 MiB)

3 of 529 groups · 775 processes · total 20.3 GiB of 24.0 GiB RAM
metric: memory footprint (same as Activity Monitor); 252 unreadable processes counted via RSS
```

Compared to `ps` or `top`, memhogs fixes four things:

1. Sorted by memory, descending. (`ps axo rss,comm -r` sorts by CPU,
   which has burned everyone at least once.)
2. Real application names. Warp's executable is literally called
   `stable`; memhogs resolves it to Warp through its `.app` bundle on
   macOS, or through the systemd unit on Linux.
3. Helpers roll up into their app by walking the process tree, not by
   matching names. A `tsserver` spawned by your editor's extension host
   counts toward the editor even though nothing in its path mentions
   the editor.
4. Honest numbers. The default metric does not double-count memory
   shared between processes, which inflates `ps`-style totals by
   gigabytes on Electron-heavy machines. See "How memory is measured".

Works on macOS and Linux, amd64 and arm64.

## Install

Prebuilt binaries, no Go required:

```sh
# Homebrew (macOS and Linux)
brew install cicerothoma/tap/memhogs

# or add the tap once, then use the short name from then on
brew tap cicerothoma/tap
brew install memhogs

# Debian/Ubuntu: download the .deb from the releases page, then
sudo dpkg -i memhogs_*.deb

# Fedora/RHEL
sudo rpm -i memhogs-*.rpm

# Alpine
sudo apk add --allow-untrusted memhogs_*.apk
```

Tarballs for every platform are on the
[releases page](https://github.com/cicerothoma/memhogs/releases).
If you have Go installed, `go install github.com/cicerothoma/memhogs@latest`
also works.

For phones there is an Android companion app,
[memhogs-android](https://github.com/cicerothoma/memhogs-android), with
the same per-app grouping and the same fair memory metric, using Shizuku
for shell access.

## Usage

### Default view

`memhogs` with no arguments prints every group, largest first. Groups
with more than one process expand to their five biggest members plus a
folded remainder line. The %MEM column is the group's share of physical
RAM, and PROCESSES is how many processes were rolled into that row.

### One row per group

`--compact` drops the member breakdown:

```
$ memhogs --compact --top 5
    MEMORY    %MEM  PROCESSES  NAME
   7.3 GiB   30.4%         46  Dia
   2.1 GiB    8.8%         23  Visual Studio Code
 708.8 MiB    2.9%          6  Slack
 475.0 MiB    1.9%          5  VTDecoderXPCService
 412.9 MiB    1.7%          6  Warp
```

### Every member process

`--tree` lifts the five-member cap and shows the full contents of each
group. Useful when you want to know exactly which renderer inside a
browser is the 2 GiB one.

### Individual processes, no grouping

`--flat` ranks raw processes the way `top` would, with correct names
and the fair memory metric:

```
$ memhogs --flat --top 5
    MEMORY    %MEM      PID  NAME
 870.2 MiB    3.5%    12251  Dia
 540.7 MiB    2.2%    12257  Browser Helper
 481.2 MiB    2.0%    16103  Browser Helper (Renderer)
 464.0 MiB    1.9%    10609  Code Helper (Plugin)
 458.1 MiB    1.9%    80624  VTDecoderXPCService
```

### Filtering

Any extra argument is a case-insensitive substring filter on group
names (or process names with `--flat`):

```
$ memhogs messages
    MEMORY    %MEM  PROCESSES  NAME
 263.8 MiB    1.1%          2  Messages
                               ├─ 247.3 MiB  Messages [4675]
                               └─ 16.5 MiB  Messages Assistant Extension [7347]
  13.4 MiB    0.1%          3  MessagesBlastDoorService
...
```

### Limiting output

`--top N` truncates to the N largest entries. The footer still counts
everything, so you can see what the cutoff hid.

### JSON for scripting

`--json` emits the full structure: the metric in use, total RAM, and
every group with its member processes. Each process carries both the
fair metric (`mem_bytes`) and plain RSS (`rss_bytes`):

```
$ memhogs --json --top 1 messages
{
  "metric": "footprint",
  "total_ram_bytes": 25769803776,
  "groups": [
    {
      "name": "Messages",
      "kind": "app",
      "mem_bytes": 276662984,
      "mem": "263.8 MiB",
      "percent_of_ram": 1.1,
      "procs": [
        {
          "pid": 4675,
          "ppid": 1,
          "mem_bytes": 259311752,
          "rss_bytes": 246022144,
          "name": "Messages",
          "path": "/System/Applications/Messages.app/Contents/MacOS/Messages"
        }
      ]
    }
  ]
}
```

`--json --flat` gives a flat process array instead.

### Live view

`--watch` opens a full-screen live view, like `top`: it repaints in
place on an interval (two seconds by default), fits itself to the
window, and never scrolls anything into your terminal's history. The
footer says how many groups the window is hiding; enlarge it or use
`--top` to choose. Ctrl-c restores your terminal exactly as it was and
prints one last snapshot so you keep a copy. Change the pace with
`--interval`, and combine with a filter to keep an eye on one thing:

```
memhogs --watch --interval 5s chrome
```

When stdout is not a terminal, `--watch` appends plain timestamped
frames instead, which makes it a cheap memory logger:
`memhogs --watch --interval 60s --compact >> mem.log`.

### Colors

When stdout is a terminal, output is colored: memory values in amber,
app groups in cyan, standalone groups in green, and any entry using 15%
or more of RAM gets its %MEM cell flagged in red. Colors turn off
automatically when piping, with `--no-color`, or when the `NO_COLOR`
environment variable is set. JSON output is never colored.

### RSS mode

`--rss` switches every view to resident set size, the number `ps` and
`top` report. Use it when you need to compare against those tools. The
footer changes to flag the caveat:

```
metric: RSS (shared memory counted once per process, so totals overstate)
```

## How memory is measured

The default metric charges shared memory fairly, so summing a group's
processes does not count the same physical pages more than once:

- On macOS it reads each process's physical memory footprint through
  `proc_pid_rusage`. This is the number Activity Monitor's Memory
  column shows, and it includes compressed memory, which RSS misses.
- On Linux it reads PSS from `/proc/<pid>/smaps_rollup`. A page shared
  by N processes counts 1/N toward each.

The two metrics disagree with RSS in both directions. On a typical
Electron-heavy Mac, Docker dropped from 947 MiB (RSS) to 411 MiB
(footprint) because RSS was double-counting shared framework pages,
while Dia rose from 6.3 GiB to 6.8 GiB because footprint sees its
compressed memory.

Processes the OS refuses to expose (typically ones owned by other
users, including most system daemons) fall back to RSS individually,
and the footer says how many. Run with sudo for full coverage.

## How grouping works

Each process is assigned to a group by walking up its parent chain:

1. If the process or one of its ancestors is recognized as an
   application, it joins that group. On macOS recognition means the
   executable lives inside a `.app` bundle; the outermost bundle wins,
   so nested helper bundles fold into the parent app. On Linux it means
   the cgroup is an `app-*.scope` or `*.service` systemd unit.
2. The walk stops if it would cross an interactive shell (zsh, bash,
   fish, and friends). Whatever you launched from that shell becomes
   its own group instead of inflating the terminal app, and its child
   processes still roll up into it. A 4 GiB training job started from a
   Warp tab shows up as `python3`, not as Warp.
3. Anything left stands alone under its own name, and same-named
   standalone groups merge, so twenty `mdworker_shared` instances make
   one row.

## Caveats

- Memory moves. Totals can shift by gigabytes between runs minutes
  apart; use `--watch` to see it happen.
- On Linux, reading other users' executable paths and PSS requires
  root. Without it, grouping falls back to cgroup units and comm names,
  and memory falls back to RSS for those processes.
- A macOS binary built with `CGO_ENABLED=0` cannot read footprint and
  reports RSS for everything. The released binaries are built with cgo.

## Building from source

```sh
go build -o memhogs .
GOOS=linux GOARCH=amd64 go build -o memhogs-linux .   # cross-compile
```

There are no dependencies outside the Go standard library. The macOS
build uses cgo for the footprint syscall. Releases are cut with
`goreleaser release --clean` on a Mac, since the darwin build cannot be
cross-compiled with cgo from Linux.

## License

MIT
