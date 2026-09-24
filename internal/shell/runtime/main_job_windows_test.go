//go:build windows

package runtime

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// endJobsWithTheTestBinary puts this test binary in a Job Object that ends everything in
// it when the binary ends, however it ends. A job the tests start is a process that
// outlives its shell, as it should, and so it outlived a test binary that timed out or
// crashed too: one left from a hung run on 2026-09-24 was still reading /dev/zero six
// hours later, a core to itself. The job processes are in this Job Object because a child
// joins its parent's; the one each of them gets for KILL (jobTree) nests inside it.
//
// The handle is kept open on purpose. Closing it is what this process's exit does, and
// that is the moment everything in it should go.
func endJobsWithTheTestBinary() {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return
	}
	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		_ = windows.CloseHandle(job)
		return
	}
	if err := windows.AssignProcessToJobObject(job, windows.CurrentProcess()); err != nil {
		_ = windows.CloseHandle(job)
	}
}
