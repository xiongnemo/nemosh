//go:build !windows

package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
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
