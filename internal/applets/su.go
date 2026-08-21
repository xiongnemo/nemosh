package applets

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// su runs a shell with elevated privileges, which on Windows means a shell
// launched through ShellExecuteEx with the `runas` verb.
//
// The name is busybox-w32's, and so is the shape: `loginutils/suw32.c`, an
// applet odd-named from `suw32` and built only under PLATFORM_MINGW32. It is not
// POSIX -- checked against POSIX.1-2024, which specifies `newgrp` and has no
// `su` page at all -- so there is no standard to conform to and the reference is
// the whole of the specification.
//
// The important thing it does *not* do is elevate a command inside the current
// pipeline. ShellExecuteEx cannot pass handles to the child, so a redirection or
// a pipe written around `su` cannot reach the elevated shell; what it launches is
// a shell, and `-c` runs the command in that shell.
//
// It does run in the window you are already in, which busybox's does not. That
// is not the launcher's doing -- ShellExecuteEx still cannot pass a handle -- but
// the child's: it is started with no console and joins this one. See
// cmd/nemosh/attach_console_windows.go and docs/support-matrix.md, Elevation.
func newSuApplet() Applet {
	return simpleApplet{name: "su", runContext: func(ctx context.Context, args []string, _ io.Reader, _, stderr io.Writer) error {
		plan, err := planElevation(args, ProcessViewFromContext(ctx), currentConsole())
		if err != nil {
			return err
		}
		return runElevated(ctx, plan)
	}}
}

// elevationPlan is everything the launch needs, decided before anything runs.
// Separated from the launch so the argument assembly -- which is where the
// mistakes are -- can be read and tested without starting a process.
type elevationPlan struct {
	// program is the executable ShellExecuteEx is given.
	program string
	// arguments is its command line, already quoted, because ShellExecuteEx
	// takes one string rather than a vector.
	arguments string
	// directory is where the child should start. ShellExecuteEx otherwise puts
	// a process launched from a system directory in %SYSTEMROOT%\System32
	// (suw32.c:96-102), which is never where the user was.
	directory string
	// wait says whether to wait for the shell and report its status. Without it
	// su returns as soon as the shell is launched, as busybox's does.
	wait bool
	// test runs the whole path with the `open` verb instead of `runas`: no
	// elevation, no consent dialog, and therefore testable. busybox has the same
	// option and for the same reason (suw32.c:88-91).
	test bool
	// inPlace runs the elevated shell in the console this one is already in,
	// rather than in a window of its own.
	//
	// This is the difference from busybox, which always gives its shell a new
	// window (suw32.c sets nShow to SW_SHOWNORMAL and does nothing further). A
	// new console is a *plain* console: nothing has enabled virtual terminal
	// processing in it, so a shell that draws in colour draws escape codes
	// instead, and the size, font and scrollback are not the ones being used.
	//
	// It is not the launcher that makes this possible -- ShellExecuteEx still
	// cannot pass a handle. The child joins. See attach_console_windows.go.
	inPlace bool
}

// planElevation parses the options and assembles the command line.
//
// busybox's usage, which this follows:
//
//	su [-tW] [-N|-s SHELL] [root]
//	su [-tW] [-N|-s SHELL] -c CMD_STRING [[--] root [ARG0 [ARG...]]]
func planElevation(args []string, view ProcessView, console consoleHandover) (elevationPlan, error) {
	options, operands, err := parseAppletOptions(args, "tWN", "cs")
	if err != nil {
		return elevationPlan{}, err
	}
	// busybox spells this constraint into getopt32 as "s--N:N--s": -N is an
	// option of *this* shell, so it cannot be handed to a foreign one.
	if options.has('s') && options.has('N') {
		return elevationPlan{}, fmt.Errorf("-s and -N cannot be used together: -N is an option of this shell, and -s names another")
	}
	plan := elevationPlan{
		// -t implies -W, because a test that does not wait observes nothing.
		test: options.has('t'),
		wait: options.has('W') || options.has('t'),
	}
	// In place unless another shell was named. A foreign program cannot be told
	// to join a console -- that instruction is an option of this shell -- so -s
	// keeps the window busybox would have given it.
	plan.inPlace = !options.has('s') && console.usable()
	if plan.inPlace {
		// Waiting is not optional here, it is what keeps the two shells from
		// reading the same keyboard. The one that launched must sit still until
		// the elevated one is done.
		plan.wait = true
	}
	// The only user is root, and root here is not an account: it is the name
	// this shell reports for an elevated token, the same fiction busybox tells
	// (`id -u` answers 0 when elevated). Any other name is refused rather than
	// quietly ignored, which is busybox's behaviour at suw32.c:57-61.
	// The name is consumed either way and only checked outside test mode, which
	// is busybox's own ordering (suw32.c:56-62) -- so a test run can name
	// anything and still exercise the rest.
	if len(operands) > 0 {
		if !plan.test && operands[0] != "root" {
			return elevationPlan{}, fmt.Errorf("unknown user %q: this shell can only elevate, and calls that root", operands[0])
		}
		operands = operands[1:]
	}
	if plan.directory, err = elevationDirectory(view); err != nil {
		return elevationPlan{}, err
	}
	if options.has('s') {
		plan.program = options.value('s')
		plan.arguments = joinWindowsArguments(foreignShellArguments(plan.program, options, operands))
		return plan, nil
	}
	if plan.program, err = os.Executable(); err != nil {
		return elevationPlan{}, fmt.Errorf("cannot find this shell's own program: %w", err)
	}
	plan.arguments = joinWindowsArguments(append(ownShellArguments(options, plan, console), operands...))
	return plan, nil
}

// consoleHandover is what this process can offer the elevated shell: whether
// there is a console at all, and which process owns it. An interface so the
// planner stays testable without one -- a test runner frequently has none.
type consoleHandover interface {
	usable() bool
	ownerProcessID() int
}

// ownShellArguments is how this shell is asked for what su was asked for.
//
// `-i` because what is being asked for is a shell to work in: a nemosh with no
// script operand reads stdin, and an elevated process launched this way has no
// stdin worth reading.
//
// `-N` leads, because the dispatch in cmd/nemosh reads `-c` by position. It is
// also the child that has to honour it: the new console belongs to that process
// and dies with it, so nothing on this side could hold it open.
func ownShellArguments(options appletOptions, plan elevationPlan, console consoleHandover) []string {
	var arguments []string
	if plan.inPlace {
		arguments = append(arguments, "--attach-console", strconv.Itoa(console.ownerProcessID()))
	}
	// -N holds a console open so its output is not lost when it closes. Nothing
	// closes when the shell runs in the console it was called from, so asking
	// for it there would only add a keypress between the command and the prompt.
	if options.has('N') && !plan.inPlace {
		arguments = append(arguments, "-N")
	}
	if options.has('c') {
		return append(arguments, "-c", options.value('c'))
	}
	return append(arguments, "-i")
}

// foreignShellArguments builds the tail for a shell named with -s.
//
// `cmd.exe` takes /c where a POSIX shell takes -c, which busybox special-cases
// by basename (suw32.c:118-120). Nothing else about a foreign shell is assumed.
func foreignShellArguments(shell string, options appletOptions, operands []string) []string {
	var arguments []string
	if options.has('c') {
		flag := "-c"
		if base := windowsBaseName(shell); base == "cmd.exe" || base == "cmd" {
			flag = "/c"
		}
		arguments = append(arguments, flag, options.value('c'))
	}
	return append(arguments, operands...)
}

// windowsBaseName cuts at either separator, folded, rather than asking
// path/filepath.
//
// filepath.Base answers for the *host*, and this is a question about the
// launch: the path is a Windows path whatever the code was compiled for. On
// Linux it left `C:\Windows\System32\cmd.exe` whole, so cmd.exe went unnoticed
// and was handed `-c`, which it does not take. The Unix runners caught it; a
// Windows-only test never would have.
func windowsBaseName(path string) string {
	if cut := strings.LastIndexAny(path, `/\`); cut >= 0 {
		path = path[cut+1:]
	}
	return strings.ToLower(path)
}

// elevationDirectory is the working directory to hand the child, canonicalised
// now rather than passed as the shell sees it.
//
// busybox canonicalises for a reason worth keeping: a directory reached through
// a mapped network drive may not exist under the elevated token, because drive
// mappings belong to a logon session (suw32.c:103-113).
func elevationDirectory(view ProcessView) (string, error) {
	if view == nil {
		return os.Getwd()
	}
	resolved, err := ResolveProcessPath(view, ".")
	if err != nil {
		return "", fmt.Errorf("cannot resolve the current directory: %w", err)
	}
	if resolved.Device {
		return "", fmt.Errorf("the current directory is a device, which a shell cannot start in")
	}
	return resolved.Native, nil
}

// joinWindowsArguments turns a vector into the single string ShellExecuteEx
// takes, quoted so CommandLineToArgvW -- which is what the child will use to
// split it again -- gives back exactly what went in.
//
// This is the rule Microsoft documents for that function and the one busybox's
// quote_arg implements: backslashes are literal except before a quote, where
// they double, and a run of them at the end of a quoted argument doubles too.
func joinWindowsArguments(arguments []string) string {
	quoted := make([]string, 0, len(arguments))
	for _, argument := range arguments {
		quoted = append(quoted, quoteWindowsArgument(argument))
	}
	return strings.Join(quoted, " ")
}

func quoteWindowsArgument(argument string) string {
	if argument == "" {
		// An empty argument has to be spelled, or it disappears from the vector
		// the child rebuilds.
		return `""`
	}
	if !strings.ContainsAny(argument, " \t\n\v\"") {
		return argument
	}
	var quoted strings.Builder
	quoted.WriteByte('"')
	slashes := 0
	for index := 0; index < len(argument); index++ {
		character := argument[index]
		switch character {
		case '\\':
			slashes++
		case '"':
			// The run of backslashes already written is doubled by writing it
			// again, and one more escapes the quote itself. Without this,
			// `a\"b` would arrive as `a"b`.
			quoted.WriteString(strings.Repeat(`\`, slashes+1))
			slashes = 0
		default:
			slashes = 0
		}
		quoted.WriteByte(character)
	}
	// A trailing run would otherwise escape the closing quote instead of being
	// data, so it is doubled too.
	quoted.WriteString(strings.Repeat(`\`, slashes))
	quoted.WriteByte('"')
	return quoted.String()
}
