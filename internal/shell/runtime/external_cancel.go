package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// What Ctrl-C does to the program the shell is waiting for. The console sends it to every
// process on it, the program too, which is how a program gets to handle it: ping prints its
// statistics, a nested cmd or pwsh stops the command it runs and stays, python unwinds. The
// shell is interrupted as well, and it ended the program, which busybox-w32 does too, measured
// in cmd/nemosh/ctrlc_console_windows_test.go: whatever the program was doing about the Ctrl-C
// was cut off, and a python, cmd or pwsh at the prompt was closed with what it was running.
//
// A program run from the prompt is left to it now, as cmd leaves it and bash does, and the shell
// waits for it to end; one in a script is ended as busybox ends it, so that Ctrl-C stops a
// script at once. NEMOSH_CTRL_C=program leaves it to the program everywhere, and kill ends it
// everywhere, as before. A program that is not on the console, which the Ctrl-C did not reach,
// is ended whatever is set; so is any program the shell stops for another reason -- timeout, a
// pipe no one reads, kill -- whose cause is not the console's.
const ctrlCSetting = "NEMOSH_CTRL_C"

// leaveToProgram is whether a program the shell's context was cancelled for is left to end by
// itself: the cancel was the console's Ctrl-C, the program has it too, and the setting says so.
func leaveToProgram(cause error, setting string, shares bool) bool {
	interrupt, console := errors.AsType[consoleInterrupt](cause)
	if !console || !shares {
		return false
	}
	switch setting {
	case "kill":
		return false
	case "program":
		return true
	}
	return interrupt.prompt
}

// armForegroundCancel is what cancelling ctx does to cmd: it is ended, unless leaveToProgram.
// Then the cancel says the process is done, which os/exec takes as nothing to report, and Wait
// returns what the program ended with.
func (r Runtime) armForegroundCancel(ctx context.Context, cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		if leaveToProgram(context.Cause(ctx), r.vars[ctrlCSetting], childSharesConsole(cmd.Process.Pid)) {
			return fmt.Errorf("left to end by itself: %w", os.ErrProcessDone)
		}
		return cmd.Process.Kill()
	}
}
