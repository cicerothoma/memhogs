# Product Hunt launch assets

Media and copy for the memhogs Product Hunt launch.

## Images

| File | Size | Use |
|------|------|-----|
| `thumbnail.png` | 240×240 | Listing thumbnail / logo |
| `thumbnail@2x.png` | 480×480 | Hi-res thumbnail for other placements |
| `gallery-1-demo.gif` | 1270×760 | Gallery #1 — autoplays in the feed (tree → compact → filter) |
| `gallery-2-compact.png` | 1276×756 | Gallery #2 — apps ranked, helpers rolled in |
| `gallery-3-flat.png` | 1276×756 | Gallery #3 — raw processes, the mess memhogs untangles |
| `gallery-4-watch.png` | 1276×756 | Gallery #4 — full-screen live view |

Upload the gallery items in the numbered order; lead with the GIF so it autoplays.

### Suggested gallery captions

- **demo** — One tool, three views: grouped tree, one-line-per-app, and filtered.
- **compact** — Every app ranked by real memory use, helpers already rolled in.
- **flat** — Raw processes the way top shows them. This is the mess memhogs cleans up.
- **watch** — Live view, like top: repaints in place, fits your window.

## Copy

**Tagline** (≤60 chars): See which apps are really eating your RAM

**Description:**

> `top` shows you 40 Chrome processes and zero answers. memhogs is a
> command-line tool that groups every helper and child process under the app
> that owns them — so an Electron app with 40 renderers is one row with one
> honest total, not 40 scattered lines. It resolves real app names (Warp's
> binary is literally called `stable`), and it charges shared memory fairly
> instead of double-counting it the way `ps` does. macOS and Linux, one
> `brew install`.

## Regenerating

- **Thumbnail:** edit `src/thumbnail.svg`, then
  `rsvg-convert -w 240 -h 240 src/thumbnail.svg -o thumbnail.png`
  (and `-w 480 -h 480 … -o thumbnail@2x.png`). Needs `librsvg` (`brew install librsvg`).
- **Gallery stills:** the `src/*.tape` files drive [vhs](https://github.com/charmbracelet/vhs).
  Each writes a directory of frames; the final still is the highest-numbered
  `frame-text-*.png` in it. Needs `memhogs` on `PATH`.
- **Demo GIF:** `vhs ../demo.tape` (renders `../demo.gif`, which the README and site also use).
