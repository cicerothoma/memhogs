// Watch mode: a full-screen live view in the terminal's alternate screen
// buffer, like top. Each refresh repaints in place, clipped to the window,
// so nothing ever scrolls into the shell's history; on ctrl-c the terminal
// is restored and a final snapshot is printed normally so a copy survives.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/cicerothoma/memhogs/internal/group"
	"github.com/cicerothoma/memhogs/internal/render"
)

func watchLoop(hooks group.Hooks, opts render.Opts, filter string, flat, rss bool, interval time.Duration) {
	tty := stdoutIsTTY()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	if tty {
		fmt.Print("\x1b[?1049h\x1b[?25l") // enter alternate screen, hide cursor
	}
	restore := func() {
		if tty {
			fmt.Print("\x1b[?1049l\x1b[?25h")
		}
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := drawFrame(hooks, opts, filter, flat, rss, interval, tty); err != nil {
			restore()
			fatal(err.Error())
		}
		select {
		case <-sig:
			restore()
			if tty {
				fmt.Printf("memhogs · last look at %s\n\n", time.Now().Format("15:04:05"))
				if err := run(os.Stdout, hooks, opts, filter, flat, false, rss, 0); err != nil {
					fatal(err.Error())
				}
			}
			return
		case <-ticker.C:
		}
	}
}

// drawFrame renders one refresh. On a TTY it repaints in place: home the
// cursor, rewrite each line erasing its remainder, then erase whatever the
// previous frame left below — no full-screen clear, so no flicker. The
// window size is re-read every frame, which also handles resizes. When
// stdout is not a TTY (piped), frames are appended plainly instead.
func drawFrame(hooks group.Hooks, opts render.Opts, filter string, flat, rss bool, interval time.Duration, tty bool) error {
	rows, cols := 0, 0
	if tty {
		rows, cols = termSize()
	}
	budget := 0
	if rows > 0 {
		budget = rows - 3 // the two header lines plus a spare row at the bottom
	}
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "memhogs · %s · every %s (ctrl-c to quit)\n\n", time.Now().Format("15:04:05"), interval)
	if err := run(&buf, hooks, opts, filter, flat, false, rss, budget); err != nil {
		return err
	}
	if !tty {
		_, err := os.Stdout.Write(buf.Bytes())
		return err
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	var out strings.Builder
	out.WriteString("\x1b[H")
	for i, l := range lines {
		if cols > 0 {
			l = clipLine(l, cols)
		}
		out.WriteString(l)
		out.WriteString("\x1b[K")
		if i < len(lines)-1 {
			out.WriteByte('\n')
		}
	}
	out.WriteString("\x1b[J")
	_, err := os.Stdout.WriteString(out.String())
	return err
}

// fitTop returns how many groups fit in budget rows, at least 1, so the
// table's own "N of M groups" footer reports what a small window hides.
func fitTop(groups []group.Group, o render.Opts, budget int) int {
	n := 0
	for _, g := range groups {
		r := groupRows(g, o)
		if budget < r {
			break
		}
		budget -= r
		n++
	}
	if n < 1 {
		n = 1
	}
	return n
}

// groupRows counts the rows a group occupies, mirroring render.Table: the
// group row, its expanded members, and the "… N more" fold line if capped.
func groupRows(g group.Group, o render.Opts) int {
	if !o.Tree || len(g.Procs) <= 1 {
		return 1
	}
	if o.MaxMembers > 0 && o.MaxMembers < len(g.Procs) {
		return 1 + o.MaxMembers + 1
	}
	return 1 + len(g.Procs)
}

// clipLine truncates a line to width visible columns, passing ANSI escape
// sequences through untouched so colors stay balanced across the cut.
func clipLine(s string, width int) string {
	if len(s) <= width { // bytes ≥ visible columns, so short lines pass as-is
		return s
	}
	var b strings.Builder
	visible, esc := 0, false
	for _, r := range s {
		switch {
		case esc:
			b.WriteRune(r)
			if r >= '@' && r <= '~' && r != '[' {
				esc = false
			}
		case r == '\x1b':
			esc = true
			b.WriteRune(r)
		case visible < width:
			b.WriteRune(r)
			visible++
		}
	}
	return b.String()
}

// termSize reads the window size from the tty; zeros mean "unknown".
func termSize() (rows, cols int) {
	var sz struct{ rows, cols, xpix, ypix uint16 }
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, os.Stdout.Fd(), syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&sz)))
	if errno != 0 {
		return 0, 0
	}
	return int(sz.rows), int(sz.cols)
}
