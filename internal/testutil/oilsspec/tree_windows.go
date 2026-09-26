package oilsspec

import (
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// processTree is a case's shell and everything it starts, held in a Job Object so that a
// case that runs out of time, or leaves a job running when it ends, can be ended whole.
// Closing the object ends what is left in it.
//
// The shell starts suspended and is resumed once it is held, so it cannot start a process
// before it joins. A process it started before would never join, and on a loaded machine,
// where this goroutine can wait a while between starting the shell and holding it, would
// run on past its case.
type processTree struct {
	job windows.Handle
	// mu guards pid and held, which exec's own goroutine reads when it ends a case that
	// ran out of time.
	mu   sync.Mutex
	pid  int
	held bool
}

func newProcessTree(cmd *exec.Cmd) (*processTree, error) {
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
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED}
	return &processTree{job: job}, nil
}

// attach puts the started shell in the tree and lets it run. What it starts from then on
// joins by itself.
func (t *processTree) attach(pid int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pid = pid
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_SUSPEND_RESUME, false, uint32(pid))
	if err != nil {
		resumeThreads(pid)
		return
	}
	defer windows.CloseHandle(process)
	t.held = windows.AssignProcessToJobObject(t.job, process) == nil
	if status, _, _ := ntResumeProcess.Call(uintptr(process)); status != 0 {
		resumeThreads(pid)
	}
}

// ntResumeProcess resumes every thread of a process in one call. ntdll exports it without
// documenting it, and every Windows since XP has it; resumeThreads, the documented way,
// takes a snapshot of every thread on the machine, which made a run of the suite eight
// times slower when every case paid for one.
var ntResumeProcess = windows.NewLazySystemDLL("ntdll.dll").NewProc("NtResumeProcess")

// resumeThreads lets the threads of a process started suspended run: its first, and the
// only one it has had the chance to have.
func resumeThreads(pid int) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	for err := windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != uint32(pid) {
			continue
		}
		if thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID); err == nil {
			_, _ = windows.ResumeThread(thread)
			_ = windows.CloseHandle(thread)
		}
	}
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
