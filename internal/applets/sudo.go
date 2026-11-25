package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"syscall"

	"github.com/xiongnemo/nemosh/internal/sudo"
)

// sudo runs a command with administrator rights in an SSH session this shell runs without them:
// the session's elevated nemosh, which started it, runs the command with this sudo's streams,
// directory and environment, and answers with its status. See package sudo, and in the runtime,
// ssh_session_windows.go.
//
//	sudo [-iEns] [-u root] [COMMAND [ARG]...]
//
// With no command, or -s or -i, it runs a shell. -E and -n are taken and change nothing: the
// environment always goes with the command, and there is no password to ask for, the session's key
// having been it. -u names no user but root, the name this shell gives an elevated token.
//
// Where no session lends, sudo is the PATH's, Windows's own sudo.exe, given every argument as it
// came; with none either, that is said.
type sudoApplet struct{}

func newSudoApplet() Applet     { return sudoApplet{} }
func (sudoApplet) Name() string { return "sudo" }

// sudoProcessVariable marks the process sudo runs itself in, so that it runs itself once at most.
const sudoProcessVariable = "NEMOSH_SUDO_PROCESS"

// errNothingLendsSudo is made where it is said, not at every start of the shell.
func errNothingLendsSudo() error {
	return errors.New("no elevated nemosh lends sudo here: one does in an SSH session it started without administrator rights, and there is no sudo on PATH")
}

func (sudoApplet) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	view := ProcessViewFromContext(ctx)
	pipe, _ := view.LookupEnv(sudo.PipeVariable)
	if pipe == "" {
		if program, ok := programFor(ctx, "sudo"); ok {
			return program.Run(ctx, args, stdin, stdout, stderr)
		}
		return ExitStatusMessage(1, errNothingLendsSudo())
	}
	options, operands, err := parseAppletOptionsInOrder(args, "iEns", "u")
	if err != nil {
		return err
	}
	if options.has('u') && options.value('u') != "root" {
		return fmt.Errorf("unknown user %q: this session lends only its elevation, which this shell calls root", options.value('u'))
	}
	streams, release, ok := sudoStreams(ctx, stdin, stdout, stderr)
	if ok {
		defer release()
	} else {
		// The lease goes back first: the process run next takes stdin as a program does.
		release()
		// Run once more, and no more: a sudo that finds no handles in a process of its own would
		// otherwise start itself until the machine ran out of memory.
		if _, again := view.LookupEnv(sudoProcessVariable); again {
			return fmt.Errorf("cannot take this process's streams (%T, %T, %T)", stdin, stdout, stderr)
		}
		self, err := os.Executable()
		if err != nil {
			return fmt.Errorf("cannot find this shell's own program: %w", err)
		}
		program, ok := programFor(ctx, self)
		if !ok {
			return commandNotFound(self)
		}
		marked := newStaticProcessView(view, view.Environ())
		marked.set(sudoProcessVariable, "1")
		return program.Run(WithProcessView(ctx, marked), append([]string{"sudo"}, args...), stdin, stdout, stderr)
	}
	directory, err := ResolveProcessPath(view, ".")
	if err != nil {
		return err
	}
	// The mark stays with this process: a sudo inside the command may run itself again.
	env := slices.DeleteFunc(view.Environ(), func(entry string) bool {
		return strings.HasPrefix(strings.ToUpper(entry), sudoProcessVariable+"=")
	})
	request := sudo.Request{Shell: options.has('s') || options.has('i') || len(operands) == 0,
		Dir: directory.Native, Args: operands, Env: env}
	for index, stream := range streams {
		if handle := stream.Fd(); handle != uintptr(syscall.InvalidHandle) {
			request.Handles[index] = uint64(handle)
		}
	}
	reply, err := sudo.Ask(pipe, request)
	if err != nil {
		return ExitStatusMessage(1, err)
	}
	if reply.Message != "" {
		fmt.Fprintf(stderr, "sudo: %s\n", reply.Message)
	}
	if reply.Status != 0 {
		return ExitStatus(int(reply.Status))
	}
	return nil
}

// sudoStreams is the three streams as files of this process, which the lender can take, when they
// all end at one: the console or a file the shell redirected to, or in a process of sudo's own its
// standard handles. A stream that is the shell's own, a pipe to the next stage of a pipeline in
// this process, is none. Stdin is leased from under the wrapper the registry gives it.
func sudoStreams(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) ([3]*os.File, func(), bool) {
	input, release, _ := leaseTopStdin(ctx, stdin)
	output, errorOutput := stdoutFile(stdout), stdoutFile(stderr)
	return [3]*os.File{input, output, errorOutput}, release, input != nil && output != nil && errorOutput != nil
}
