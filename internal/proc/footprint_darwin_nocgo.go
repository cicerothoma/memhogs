//go:build darwin && !cgo

package proc

// Without cgo there is no libproc access; every process falls back to RSS.
func footprint(pid int) uint64 { return 0 }
