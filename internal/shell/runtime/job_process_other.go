//go:build !windows

package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// prepareJobCommand passes the state pipe as the child's descriptor 3, and makes the child
// its own process group so a terminal's Ctrl-C reaches the foreground and not the job.
func prepareJobCommand(command *exec.Cmd, state *os.File) (string, error) {
	command.ExtraFiles = []*os.File{state}
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return "3", nil
}

// JobStateFile is the child's end of the state pipe, from the argument after --job.
func JobStateFile(argument string) (*os.File, error) {
	if argument != "3" {
		return nil, fmt.Errorf("--job %s: not the state descriptor", argument)
	}
	return os.NewFile(3, "job-state"), nil
}
