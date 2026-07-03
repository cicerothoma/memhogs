# memhogs

Lists applications and services by memory use, largest first — with helper
and child processes rolled up into the app that owns them.

Born from a frustrating session with raw `ps`:

- `ps ... -r` sorts by CPU, not memory. memhogs sorts by memory, always.
- Binary names hide the real app (Warp's executable is literally named
  `stable`). memhogs resolves the owning `.app` bundle on macOS and the
  systemd unit on Linux.
- Electron apps scatter memory across dozens of helpers. memhogs aggregates
  them by walking the process tree, so a language server spawned by your
  editor's extension host counts toward the editor even though its own
  path never mentions the app.
- Things *you* launch from a terminal do **not** roll into the terminal:
  the tree walk stops at interactive shells, so your 4 GB python job shows
  up as `python3`, not as Warp.

## Usage

```
memhogs                 # groups sorted by memory, top 5 members shown per group
memhogs --tree          # expand every member process, not just the top 5
memhogs --compact       # one row per group, no member breakdown
memhogs --flat          # per-process view, no grouping
memhogs --top 10        # only the 10 largest
memhogs --json          # machine-readable output
memhogs --watch         # refresh every 2s (--interval 5s to change)
memhogs --rss           # ps/top-comparable RSS instead of the fair metric
memhogs cursor          # only groups matching "cursor"
```

## Installing

No Go required — prebuilt binaries for macOS and Linux (amd64/arm64):

```
# Homebrew (macOS and Linux)
brew install cicerothoma/tap/memhogs

# Debian/Ubuntu — grab the .deb from the latest release, then:
sudo dpkg -i memhogs_*.deb

# Fedora/RHEL
sudo rpm -i memhogs-*.rpm

# Alpine
sudo apk add --allow-untrusted memhogs_*.apk

# Or download a tarball from https://github.com/cicerothoma/memhogs/releases
```

With Go installed, `go install github.com/cicerothoma/memhogs@latest` works too.

## Building from source

```
go build -o memhogs .
GOOS=linux GOARCH=amd64 go build -o memhogs-linux .   # cross-compile
```

Releases are cut locally with `goreleaser release --clean` (the macOS build
needs cgo, so releases run on a Mac).

No dependencies beyond the Go standard library. macOS and Linux. The macOS
build uses cgo (Apple's libproc) to read memory footprint; a `CGO_ENABLED=0`
build still works but falls back to RSS for every process.

## Memory metrics

By default memhogs reports a **fair-share metric**, so summing a group's
processes doesn't double-count shared pages:

- **macOS:** physical memory footprint via `proc_pid_rusage` — the same
  number Activity Monitor's Memory column shows.
- **Linux:** PSS from `/proc/<pid>/smaps_rollup`, which charges each shared
  page fractionally to the processes sharing it.

Processes the OS won't let us inspect (typically other users' processes;
run as root/sudo for full coverage) fall back to RSS per process — the
footer reports how many. `--rss` switches everything to plain RSS when you
want numbers comparable to `ps`/`top`; note RSS counts shared pages once
*per process*, so grouped totals overstate real usage.

## Caveats

- Memory fluctuates: totals can shift by gigabytes between runs minutes
  apart. Use `--watch` to see movement.
- On Linux, executable paths and PSS of other users' processes aren't
  readable without root; grouping falls back to cgroup units and comm
  names, and memory falls back to RSS.

## How grouping works

Each process is assigned to a group by walking its parent chain:

1. If the process (or an ancestor) is recognized as an app — its executable
   lives in a `.app` bundle (macOS) or its cgroup is an `app-*.scope` /
   `*.service` unit (Linux) — it joins that app's group. The *outermost*
   bundle wins, so nested helper bundles fold into the parent app.
2. If the walk would cross an interactive shell (zsh, bash, fish, …), it
   stops: the topmost process below the shell becomes a standalone group,
   and its own children roll up into it.
3. Otherwise the process stands alone under its own name; same-named
   standalone groups merge (e.g. all `mdworker_shared` instances).
