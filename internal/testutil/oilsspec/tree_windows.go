package oilsspec

import (
	"os/exec"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// processTree is a case's shell and everything it starts, held in a Job Object so that a
// case that runs out of time, or leaves a job running when it ends, can be ended whole.
// Closing the object ends what is left in it.
type processTree struct {
	job windows.Handle
	// mu guards pid and held, which exec's own goroutine reads when it ends a case that
	// ran out of time.
	mu   sync.Mutex
	pid  int
	held bool
}

func newProcessTree(*exec.Cmd) (*processTree, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		_ = windows.CloseHandle(job)
		return nil, err
	}
	return &processTree{job: job}, nil
}

// attach puts the started shell in the tree. What it starts from then on joins by
// itself; it has not had the time to start anything before. A shell that has already
// ended cannot join, and is not held.
func (t *processTree) attach(pid int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pid = pid
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return
	}
	defer windows.CloseHandle(process)
	t.held = windows.AssignProcessToJobObject(t.job, process) == nil
}

func (t *processTree) kill() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.held {
		return windows.TerminateJobObject(t.job, 1)
	}
	process, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(t.pid))
	if err != nil {
		return nil // it has ended
	}
	defer windows.CloseHandle(process)
	return windows.TerminateProcess(process, 1)
}

func (t *processTree) close() {
	_ = windows.CloseHandle(t.job)
}
