package applets

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
)

// :!COMMAND and :r !COMMAND, as busybox's system() runs them, `sh -c COMMAND`, through the shell
// vi was started from, which is this one: an applet never starts a process (program_runner.go).
// % in COMMAND is the file's name and # the one before, as busybox's expand_args has them. :!
// gives the console back while it runs, and waits for Enter after, busybox's Hit_Return.

// expandFileNames is busybox's expand_args: % and #, and a backslash before either for itself.
func (e *viEditor) expandFileNames(command string) (string, bool) {
	var out strings.Builder
	for index := 0; index < len(command); index++ {
		switch char := command[index]; {
		case char == '\\' && index+1 < len(command) && (command[index+1] == '%' || command[index+1] == '#'):
			index++
			out.WriteByte(command[index])
		case char == '%' && e.file.name == "":
			e.say(true, "No current filename")
			return "", false
		case char == '%':
			out.WriteString(e.file.name)
		case char == '#' && e.alternate == "":
			e.say(true, "No previous filename")
			return "", false
		case char == '#':
			out.WriteString(e.alternate)
		default:
			out.WriteByte(char)
		}
	}
	return out.String(), true
}

// runShell runs COMMAND with sh -c, and answers its status, or -1 when there is no sh to run.
func (e *viEditor) runShell(command string, stdout *bytes.Buffer) int {
	command, ok := e.expandFileNames(command)
	if !ok {
		return -1
	}
	// No applet is sh, so this is the shell's own program for it, which is itself; asking the
	// registry first, as commandFor does, would make the registry's table refer to itself.
	shell, found := programFor(e.ctx, "sh")
	if !found {
		e.say(true, "sh: not found")
		return -1
	}
	var input io.Reader = bytes.NewReader(nil)
	var output, errors io.Writer = io.Discard, io.Discard
	switch {
	case stdout != nil:
		output, errors = stdout, stdout
	default:
		if e.stdin != nil {
			input = e.stdin
		}
		if e.stdout != nil {
			output = e.stdout
		}
		if e.stderr != nil {
			errors = e.stderr
		}
	}
	return awkCommandStatus(shell.Run(e.ctx, []string{"-c", command}, input, output, errors))
}

// shellCommand is :!COMMAND.
func (e *viEditor) shellCommand(given int, command string) {
	if given > 0 {
		e.say(true, "Range not allowed")
		return
	}
	if e.screen != nil {
		_ = e.screen.Suspend()
	}
	status := e.runShell(command, nil)
	if status > 0 && e.stdout != nil {
		fmt.Fprintf(e.stdout, "\nshell returned %d\n\n", status)
	}
	if e.stdout != nil && e.stdin != nil && status >= 0 {
		fmt.Fprint(e.stdout, "[Hit return to continue]")
		_, _ = bufio.NewReader(e.stdin).ReadString('\n')
	}
	if e.screen != nil {
		_ = e.screen.Resume()
	}
}

// commandOutput is what COMMAND writes, as lines, or nil when it could not be run.
func (e *viEditor) commandOutput(command string) [][]rune {
	var out bytes.Buffer
	if e.runShell(command, &out) < 0 {
		return nil
	}
	lines := splitText([]rune(strings.ReplaceAll(out.String(), "\r\n", "\n")))
	if lines == nil {
		lines = [][]rune{}
	}
	return lines
}
