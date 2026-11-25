package applets

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
)

// nohup runs a command the end of the session does not end: busybox's (coreutils/nohup.c), which
// busybox-w32 does not build. A terminal on stdin is given up for the null device, a terminal on
// stdout for nohup.out, appended to here or else in $HOME, and a terminal on stderr for stdout.
//
// What ends a command with its session is not a hangup on Windows. sshd puts a session in a Job
// Object that ends everything in it when the session closes, and a console that closes ends what
// is on it. So the shell launches the command out of both, asked through the context: out of every
// Job Object that lets it leave, sshd's among them, and onto a console of its own that no one sees.
// The command is always a process of its own for that, an applet launched as this program with the
// applet's name, since one run inside the shell would end with it.
//
// It checks that the command exists before it touches nohup.out, where busybox finds out when it
// executes it, so a mistyped name leaves no file behind. Both fail with 127.
//
// A type with a method rather than a simpleApplet, as env is: it looks applets up, and a function
// value in the table would make the table's initialisation depend on itself.
type nohupApplet struct{}

func newNohupApplet() Applet     { return nohupApplet{} }
func (nohupApplet) Name() string { return "nohup" }
func (nohupApplet) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	return runNohup(ctx, args, stdin, stdout, stderr)
}

// nohupTerminals is which of stdin, stdout and stderr is a terminal; a variable so a test can say.
var nohupTerminals = func(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) [3]bool {
	return [3]bool{inputIsTerminal(ctx, stdin), stdoutIsTerminal(stdout), stdoutIsTerminal(stderr)}
}

func runNohup(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return ExitStatusMessage(127, missingOperand())
	}
	program, programArgs, ok := nohupProgram(ctx, args[0], args[1:])
	if !ok {
		return commandNotFound(args[0])
	}
	terminals := nohupTerminals(ctx, stdin, stdout, stderr)
	if terminals[0] {
		null, err := os.Open(os.DevNull)
		if err != nil {
			return err
		}
		defer null.Close()
		stdin = null
	}
	if terminals[1] {
		output, shown, err := openNohupOut(ProcessViewFromContext(ctx))
		if err != nil {
			return ExitStatusMessage(127, err)
		}
		defer output.Close()
		if shown != "" {
			fmt.Fprintf(stderr, "nohup: appending output to %s\n", shown)
		}
		stdout = output
	}
	if terminals[2] {
		stderr = stdout
	}
	return program.Run(WithDetachedLaunch(ctx), programArgs, stdin, stdout, stderr)
}

// nohupProgram is the program nohup runs for name, and its arguments: an applet is this program
// told the applet's name, so that it is a process the shell can launch out of the session.
func nohupProgram(ctx context.Context, name string, args []string) (Applet, []string, bool) {
	if _, ok := DefaultRegistry.Lookup(name); ok {
		self, err := os.Executable()
		if err != nil {
			return nil, nil, false
		}
		program, ok := programFor(ctx, self)
		return program, append([]string{name}, args...), ok
	}
	program, ok := programFor(ctx, name)
	return program, args, ok
}

// openNohupOut opens nohup.out to append to, here or else in $HOME, and answers with the name to
// say: busybox's order. With no HOME the output goes nowhere, and nothing is said of it.
func openNohupOut(view ProcessView) (*os.File, string, error) {
	if output, err := openNohupFile(view, "nohup.out"); err == nil {
		return output, "nohup.out", nil
	}
	home, ok := view.LookupEnv("HOME")
	if !ok || home == "" {
		output, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		return output, "", err
	}
	shown := strings.TrimSuffix(home, "/") + "/nohup.out"
	output, err := openNohupFile(view, shown)
	if err != nil {
		return nil, "", fmt.Errorf("can't open '%s': %w", shown, err)
	}
	return output, shown, nil
}

func openNohupFile(view ProcessView, path string) (*os.File, error) {
	resolved, err := ResolveProcessPath(view, path)
	if err != nil {
		return nil, err
	}
	return os.OpenFile(resolved.Native, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
}
