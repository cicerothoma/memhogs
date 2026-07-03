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
memhogs                 # grouped table, sorted by memory descending
memhogs --tree          # show member processes under each group
memhogs --flat          # per-process view, no grouping
memhogs --top 10        # only the 10 largest
memhogs --json          # machine-readable output
memhogs --watch         # refresh every 2s (--interval 5s to change)
memhogs cursor          # only groups matching "cursor"
```

## Building

```
go build -o memhogs .
GOOS=linux GOARCH=amd64 go build -o memhogs-linux .   # cross-compile
```

No dependencies beyond the Go standard library. macOS and Linux.

## Caveats

- **Memory metric is RSS.** Pages shared between processes (frameworks,
  Electron binaries mapped into every renderer) are counted once *per
  process*, so grouped totals can overstate real usage by roughly 10–30%
  for multi-process apps. Treat the numbers as "what `ps`/`top` would say,
  summed", not as exact physical footprint.
- Memory fluctuates: totals can shift by gigabytes between runs minutes
  apart. Use `--watch` to see movement.
- On Linux, executable paths of other users' processes aren't readable
  without root; grouping falls back to cgroup units and comm names.

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
