//go:build windows

package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

// prepareJobCommand lets the child inherit the state pipe and nothing else, and answers
// with the argument that names it. The handle is listed rather than every inheritable one
// being passed, and the child is its own process group, so the console's Ctrl-C reaches
// the foreground and not the job -- busybox's arrangement.
func prepareJobCommand(command *exec.Cmd, state *os.File) (string, error) {
	handle := syscall.Handle(state.Fd())
	if err := syscall.SetHandleInformation(handle, syscall.HANDLE_FLAG_INHERIT, syscall.HANDLE_FLAG_INHERIT); err != nil {
		return "", fmt.Errorf("mark the job state inheritable: %w", err)
	}
	command.SysProcAttr = &syscall.SysProcAttr{
		AdditionalInheritedHandles: []syscall.Handle{handle},
		CreationFlags:              syscall.CREATE_NEW_PROCESS_GROUP,
	}
	return strconv.FormatUint(uint64(handle), 10), nil
}

// JobStateFile is the child's end of the state pipe, from the argument after --job.
func JobStateFile(argument string) (*os.File, error) {
	handle, err := strconv.ParseUint(argument, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("--job %s: not a handle", argument)
	}
	return os.NewFile(uintptr(handle), "job-state"), nil
}
