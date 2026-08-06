//go:build windows

package runtime

import (
	"os/exec"
	"strings"
	"syscall"
)

// applyRawCommandLine hands the child its command line verbatim. Go otherwise
// composes one from Args with syscall.EscapeArg, whose \" escape cmd.exe reads
// as two separate tokens; os/exec.Command documents SysProcAttr.CmdLine as the
// supported way out for cmd.exe and batch files.
func applyRawCommandLine(command *exec.Cmd, line string) {
	command.SysProcAttr = &syscall.SysProcAttr{CmdLine: line}
}

// launchWorkingDirectory adapts the shell's working directory to what
// CreateProcess will accept for a child.
func launchWorkingDirectory(dir string) (string, error) {
	return childWorkingDirectory(dir, shortNativePath)
}

// shortNativePath asks Windows for a path's 8.3 spelling. The query is itself
// MAX_PATH-bound unless the path carries the extended-length prefix -- which is
// the whole reason to ask -- and the prefix comes back in the answer, so it is
// stripped again: a child inheriting a \\?\ working directory reports that
// spelling from getcwd, and Nemosh does not put it in front of users
// (docs/design/windows-execution-model.md:341).
func shortNativePath(dir string) (string, error) {
	query, err := syscall.UTF16PtrFromString(extendedLengthPath(dir))
	if err != nil {
		return "", err
	}
	buffer := make([]uint16, len(dir)+len(`\\?\UNC\`)+1)
	length, err := syscall.GetShortPathName(query, &buffer[0], uint32(len(buffer)))
	if err != nil {
		return "", err
	}
	if int(length) > len(buffer) {
		// The buffer was too small, so length is the size it wants instead.
		buffer = make([]uint16, length)
		if length, err = syscall.GetShortPathName(query, &buffer[0], uint32(len(buffer))); err != nil {
			return "", err
		}
	}
	return nativeLengthPath(syscall.UTF16ToString(buffer[:length])), nil
}

// extendedLengthPath spells an absolute native path the way the Win32 wide APIs
// want it when it may exceed MAX_PATH. A UNC path takes the \\?\UNC\ form
// because \\?\ turns off the parsing that would otherwise recognise the \\ .
func extendedLengthPath(path string) string {
	if strings.HasPrefix(path, `\\?\`) || strings.HasPrefix(path, `\\.\`) {
		return path
	}
	if rest, found := strings.CutPrefix(path, `\\`); found {
		return `\\?\UNC\` + rest
	}
	return `\\?\` + path
}

// nativeLengthPath is extendedLengthPath in reverse.
func nativeLengthPath(path string) string {
	if rest, found := strings.CutPrefix(path, `\\?\UNC\`); found {
		return `\\` + rest
	}
	if rest, found := strings.CutPrefix(path, `\\?\`); found {
		return rest
	}
	return path
}
