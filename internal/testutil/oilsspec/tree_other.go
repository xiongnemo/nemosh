//go:build !windows

package oilsspec

import (
	"os/exec"
	"sync"
	"syscall"
)

// processTree is a case's shell and everything it starts, held in a process group of its
// own so that a case that runs out of time, or leaves a job running when it ends, can be
// ended whole.
type processTree struct {
	// mu guards pid, which exec's own goroutine reads when it ends a case that ran out
	// of time.
	mu  sync.Mutex
	pid int
}

func newProcessTree(cmd *exec.Cmd) (*processTree, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return &processTree{}, nil
}

func (t *processTree) attach(pid int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pid = pid
}

func (t *processTree) kill() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.pid == 0 {
		return nil
	}
	return syscall.Kill(-t.pid, syscall.SIGKILL)
}

func (t *processTree) close() {}
