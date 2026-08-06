//go:build windows

package runtime

import (
	"os/exec"
	"syscall"
)

// applyRawCommandLine hands the child its command line verbatim. Go otherwise
// composes one from Args with syscall.EscapeArg, whose \" escape cmd.exe reads
// as two separate tokens; os/exec.Command documents SysProcAttr.CmdLine as the
// supported way out for cmd.exe and batch files.
func applyRawCommandLine(command *exec.Cmd, line string) {
	command.SysProcAttr = &syscall.SysProcAttr{CmdLine: line}
}
