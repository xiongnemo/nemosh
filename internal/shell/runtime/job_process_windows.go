//go:build windows

package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
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
