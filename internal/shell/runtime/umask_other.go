//go:build !windows

package runtime

import (
	"os"
	"os/exec"
	"sync"
	"syscall"
)

// initialFileModeMask is the umask the process was started with, as every shell's is: read by
// setting it and setting it back, once, while the package is initialised and nothing else is
// running to create a file in between.
var initialFileModeMask = processFileModeMask()

// processMask is the process's umask as this package last set it, and the lock every child
// the shell starts is started under: read-held for a child that takes the process's mask as
// it is, held for one that needs another.
var processMask = struct {
	sync.RWMutex
	value uint16
}{value: initialFileModeMask}

func processFileModeMask() uint16 {
	mask := syscall.Umask(0)
	syscall.Umask(mask)
	return uint16(mask & 0o777)
}

// setProcessFileModeMask is `umask` in a shell that owns the process's mask.
func setProcessFileModeMask(mask uint16) {
	processMask.Lock()
	defer processMask.Unlock()
	syscall.Umask(int(mask))
	processMask.value = mask
}

// startChild starts command with the shell's umask, which a child inherits from the process
// as it forks. That is the process's own, unless this is a subshell that set its own, `(umask
// 077; ssh-keygen ...)`, whose process is the shell's: then the process's mask is the
// subshell's only until the child has forked, under the lock, so no other child forks with it.
// A file the shell makes in-process meanwhile asks for its own mode's bits anyway; see
// createMode.
func (r Runtime) startChild(command *exec.Cmd) error {
	mask := r.FileModeMask()
	processMask.RLock()
	if processMask.value == mask {
		defer processMask.RUnlock()
		return command.Start()
	}
	processMask.RUnlock()
	processMask.Lock()
	defer processMask.Unlock()
	syscall.Umask(int(mask))
	defer syscall.Umask(int(processMask.value))
	return command.Start()
}

// createMode is perm as a file or directory the shell makes asks for it: without the bits its
// umask clears. The process's mask clears them too where the shell owns it; a subshell's may
// clear more, `(umask 077; echo > f)`, and it is the mode asked for that clears those. What a
// subshell's mask clears less than the process's, the process's still clears.
func (r Runtime) createMode(perm os.FileMode) os.FileMode {
	return perm &^ os.FileMode(r.FileModeMask())
}
