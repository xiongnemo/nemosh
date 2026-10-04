//go:build !windows

package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// prepareJobCommand makes the child its own process group, so a terminal's Ctrl-C reaches
// the foreground and not the job.
func prepareJobCommand(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// inheritFile passes file as the child's next descriptor after 2, and answers with its
// number. os/exec duplicates it into the child, so there is nothing of its own to release.
func inheritFile(command *exec.Cmd, file *os.File) (string, func() error, error) {
	command.ExtraFiles = append(command.ExtraFiles, file)
	return strconv.Itoa(2 + len(command.ExtraFiles)), func() error { return nil }, nil
}

// inheritedFile is the child's side: the file behind a descriptor inheritFile named.
func inheritedFile(handle, name string) (*os.File, error) {
	value, err := strconv.Atoi(handle)
	if err != nil || value < 3 {
		return nil, fmt.Errorf("%s: %q is not an inherited descriptor", name, handle)
	}
	return os.NewFile(uintptr(value), name), nil
}

// jobTree is a job process's process group, which prepareJobCommand made, so KILL ends
// the programs the job started too.
type jobTree struct {
	mu     sync.Mutex
	pgid   int
	closed bool
}

func (t *jobTree) attach(pid int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pgid = pid
}

func (t *jobTree) kill(process *os.Process, _ int) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return nil
	}
	if t.pgid > 0 {
		return syscall.Kill(-t.pgid, syscall.SIGKILL)
	}
	return process.Kill()
}

// close is the job's end; after it, kill does nothing, so a group id that has been reused
// is never signalled.
func (t *jobTree) close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closed = true
}

// processOutcome is a job process's status and the signal that ended it, if one did.
func processOutcome(state *os.ProcessState) (int, int) {
	if status, ok := state.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return 128 + int(status.Signal()), int(status.Signal())
	}
	return state.ExitCode(), 0
}

// endBySignal is a job ending by a signal it did not catch: the signal raised on itself
// with its default action back, so the parent sees what processOutcome reads.
//
// Go gives the default action back for HUP, INT and TERM only. For QUIT, ILL, FPE, SEGV and
// ABRT its runtime keeps a handler of its own, which answers one sent by kill with a stack
// dump and status 2, and a PIPE no write caused it ignores. For those the process becomes
// a shell that sends the signal to itself: exec keeps the pid, and gives every handled
// signal its default action. 128+n is what is left if neither ends it.
func endBySignal(number int) {
	signal.Reset(syscall.Signal(number))
	switch number {
	case 1, 2, 9, 15:
		_ = syscall.Kill(os.Getpid(), syscall.Signal(number))
		time.Sleep(time.Second)
	default:
		status := strconv.Itoa(128 + number)
		_ = syscall.Exec("/bin/sh", []string{"sh", "-c", "kill -" + strconv.Itoa(number) + " $$; exit " + status}, nil)
	}
	os.Exit(128 + number)
}
