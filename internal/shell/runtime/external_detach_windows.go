//go:build windows

package runtime

import (
	"errors"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// detachFromSession is the launch nohup asks for (applets.WithDetachedLaunch). The child is put
// out of every Job Object that lets it leave, when leave says to ask: an sshd session is one, and
// ends everything in it when it closes. It is also put in a process group of its own, so the
// console's Ctrl-C is not its, and on a console of its own that no one sees, so the session's
// console closing does not end it either and nothing it starts opens a window.
func detachFromSession(command *exec.Cmd, leave bool) {
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	flags := uint32(syscall.CREATE_NEW_PROCESS_GROUP | windows.CREATE_NO_WINDOW)
	if leave {
		flags |= windows.CREATE_BREAKAWAY_FROM_JOB
	}
	command.SysProcAttr.CreationFlags |= flags
}

// breakawayRefused is whether a launch failed because a Job Object the child asked to leave lets
// nothing leave it: CreateProcess's ERROR_ACCESS_DENIED for CREATE_BREAKAWAY_FROM_JOB.
func breakawayRefused(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED)
}
