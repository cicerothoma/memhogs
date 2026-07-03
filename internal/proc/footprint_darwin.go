//go:build darwin && cgo

package proc

/*
#include <libproc.h>
*/
import "C"
import "unsafe"

// footprint returns the process's physical memory footprint in bytes — the
// same number Activity Monitor's Memory column shows. It excludes shared
// file-backed pages (framework code), so footprints sum fairly across a
// group. Returns 0 when the kernel refuses (typically other users' procs).
func footprint(pid int) uint64 {
	var ri C.struct_rusage_info_v4
	if C.proc_pid_rusage(C.int(pid), C.RUSAGE_INFO_V4, (*C.rusage_info_t)(unsafe.Pointer(&ri))) != 0 {
		return 0
	}
	return uint64(ri.ri_phys_footprint)
}
