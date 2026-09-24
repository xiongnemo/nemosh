//go:build windows

package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"syscall"

	"golang.org/x/sys/windows"
)

// prepareJobCommand makes the child its own process group, so the console's Ctrl-C reaches
// the foreground and not the job -- busybox's arrangement.
func prepareJobCommand(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

// inheritFile lets the child inherit file, and answers with the handle value it arrives
// under. An inheritable duplicate rather than the handle itself, so a file the shell goes on
// using keeps its own flags; the duplicate is listed, so the child inherits it and nothing
// else, and release closes it once the child has it.
func inheritFile(command *exec.Cmd, file *os.File) (string, func() error, error) {
	process, err := syscall.GetCurrentProcess()
	if err != nil {
		return "", nil, err
	}
	var duplicate syscall.Handle
	if err := syscall.DuplicateHandle(process, syscall.Handle(file.Fd()), process, &duplicate, 0, true, syscall.DUPLICATE_SAME_ACCESS); err != nil {
		return "", nil, fmt.Errorf("make %s inheritable: %w", file.Name(), err)
	}
	command.SysProcAttr.AdditionalInheritedHandles = append(command.SysProcAttr.AdditionalInheritedHandles, duplicate)
	release := func() error { return syscall.CloseHandle(duplicate) }
	return strconv.FormatUint(uint64(duplicate), 10), release, nil
}

// inheritedFile is the child's side: the file behind a handle inheritFile named.
func inheritedFile(handle, name string) (*os.File, error) {
	value, err := strconv.ParseUint(handle, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%s: %q is not a handle", name, handle)
	}
	return os.NewFile(uintptr(value), name), nil
}

// jobTree is a job process's Job Object, which the programs it starts join too, so KILL
// can end them all -- busybox ends only the one process. The object does not kill on
// close: a job started at the top outlives the shell, as it does in both references.
type jobTree struct {
	mu     sync.Mutex
	handle windows.Handle
	closed bool
}

// attach puts the process in a new Job Object. A process that cannot be put in one is
// still ended alone, so a failure here is not the launch's.
func (t *jobTree) attach(pid int) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err == nil {
		err = windows.AssignProcessToJobObject(job, process)
		_ = windows.CloseHandle(process)
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if err != nil || t.closed {
		_ = windows.CloseHandle(job)
		return
	}
	t.handle = job
}

// kill ends the tree, with the exit code busybox gives a process it ends by signal.
func (t *jobTree) kill(process *os.Process, signal int) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil
	}
	if t.handle != 0 {
		return windows.TerminateJobObject(t.handle, uint32(signal)<<24)
	}
	return process.Kill()
}

// close is the job's end; after it, kill does nothing, so a handle value Windows has
// reused is never terminated.
func (t *jobTree) close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.closed && t.handle != 0 {
		_ = windows.CloseHandle(t.handle)
	}
	t.closed = true
}

// processOutcome is a job process's status and the signal that ended it: an exit code with
// the signal in its top byte, busybox's and jobTree's.
func processOutcome(state *os.ProcessState) (int, int) {
	code := uint32(state.ExitCode())
	status := jobExitStatus(code)
	if code >= 1<<24 && code&0xffffff == 0 {
		return status, int(code >> 24)
	}
	return status, 0
}

// endBySignal is a job ending by a signal it did not catch, in the form processOutcome reads.
func endBySignal(signal int) {
	os.Exit(signal << 24)
}
