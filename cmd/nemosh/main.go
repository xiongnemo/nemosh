package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
	"golang.org/x/term"
)

func main() {
	signals, stopSignals := notifyInterrupts()
	defer stopSignals()
	cmd := command{
		stdin:           os.Stdin,
		stdout:          os.Stdout,
		stderr:          os.Stderr,
		stdinIsTerminal: term.IsTerminal(int(os.Stdin.Fd())),
		interrupts:      signals,
	}
	if err := cmd.run(context.Background(), os.Args); err != nil {
		if status, ok := applets.StatusCode(err); ok {
			os.Exit(status)
		}
		if errors.Is(err, applets.ErrExitFalse) {
			os.Exit(1)
		}
		var status exitStatus
		if errors.As(err, &status) {
			os.Exit(int(status))
		}
		if _, reported := errors.AsType[reportedError](err); reported {
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type command struct {
	stdin           io.Reader
	stdout          io.Writer
	stderr          io.Writer
	stdinIsTerminal bool
	interrupts      <-chan os.Signal
	registry        applets.Registry
	state           *runtime.State
}

func run(ctx context.Context, args []string) error {
	cmd := command{
		stdin:           os.Stdin,
		stdout:          os.Stdout,
		stderr:          os.Stderr,
		stdinIsTerminal: term.IsTerminal(int(os.Stdin.Fd())),
	}
	return cmd.run(ctx, args)
}

func (c command) run(ctx context.Context, args []string) error {
	controller := &interruptController{}
	stop := make(chan struct{})
	if c.interrupts != nil {
		go func() {
			for {
				select {
				case <-c.interrupts:
					controller.interrupt()
				case <-stop:
					return
				}
			}
		}()
		defer close(stop)
	}
	registry := c.registry
	if applet, ok := registry.Lookup(applets.InvocationName(args)); ok {
		return c.runDirectApplet(ctx, controller, applet, args[1:])
	}
	if applet, ok := applets.DefaultRegistry.Lookup(applets.InvocationName(args)); ok {
		return c.runDirectApplet(ctx, controller, applet, args[1:])
	}

	if len(args) > 1 && args[1] == "-c" {
		if len(args) < 3 {
			fmt.Fprintln(c.stderr, "nemosh: -c requires an argument")
			return exitStatus(2)
		}
		return c.runScriptAs(ctx, controller, args[2], commandStringInvocation(args[3:]))
	}
	if len(args) > 1 && args[1] == "-i" {
		return c.runInteractive(ctx, controller)
	}

	if len(args) > 1 {
		if applet, ok := registry.Lookup(args[1]); ok {
			return c.runDirectApplet(ctx, controller, applet, args[2:])
		}
		if applet, ok := applets.DefaultRegistry.Lookup(args[1]); ok {
			return c.runDirectApplet(ctx, controller, applet, args[2:])
		}
		if !strings.HasPrefix(args[1], "-") {
			return c.runScriptFile(ctx, controller, args[1], args[2:])
		}
		if handled, err := c.infoFlag(args[1]); handled {
			return err
		}
		// A bare "-" is the POSIX spelling of "read the script from stdin".
		if args[1] != "-" {
			fmt.Fprintf(c.stderr, "nemosh: invalid option %s\n", args[1])
			return exitStatus(2)
		}
	}
	if len(args) == 1 && c.stdinIsTerminal {
		return c.runInteractive(ctx, controller)
	}

	data, err := readBoundedInput(c.stdin)
	if err != nil {
		if errors.Is(err, errInputTooLarge) {
			fmt.Fprintln(c.stderr, "nemosh: input too large")
			return exitStatus(2)
		}
		return fmt.Errorf("nemosh: read stdin: %w", err)
	}
	return c.runScript(ctx, controller, string(data))
}

func (c command) runDirectApplet(ctx context.Context, controller *interruptController, applet applets.Applet, args []string) error {
	executionCtx, clear := controller.context(ctx)
	defer clear()
	streams := runtime.Streams{Stdin: c.stdin, Stdout: c.stdout, Stderr: c.stderr}
	var view runtime.Runtime
	var err error
	if c.state == nil {
		view, err = runtime.NewRuntime(applets.DefaultRegistry, streams)
	} else {
		view, err = runtime.NewRuntimeWithState(applets.DefaultRegistry, streams, *c.state)
	}
	if err != nil {
		return fmt.Errorf("nemosh: initialize direct applet state: %w", err)
	}
	err = applet.Run(applets.WithProcessView(executionCtx, view), args, c.stdin, c.stdout, c.stderr)
	if runtime.IsShellInterrupt(executionCtx) {
		return exitStatus(130)
	}
	if err == nil {
		return nil
	}
	// The same mapping the shell uses, so a direct invocation and the same
	// command inside the shell fail identically. The error itself travels on
	// unchanged, because a caller testing it for a sentinel has to keep finding
	// one; only the reporting is added here.
	if _, message := runtime.AppletFailure(applet.Name(), err); message != "" {
		fmt.Fprintln(c.stderr, message)
		if _, carriesStatus := applets.StatusCode(err); !carriesStatus {
			return reportedError{err: err}
		}
	}
	return err
}

// reportedError marks an error whose diagnostic has already been written, so
// the top level exits with it rather than printing a second, unprefixed copy.
// It keeps wrapping the original, so errors.Is and errors.As still reach it.
type reportedError struct{ err error }

func (e reportedError) Error() string { return e.err.Error() }

func (e reportedError) Unwrap() error { return e.err }

var errInputTooLarge = errors.New("input too large")

func readBoundedInput(reader io.Reader) ([]byte, error) {
	var data bytes.Buffer
	buffer := make([]byte, 32*1024)
	emptyReads := 0
	for {
		count, err := reader.Read(buffer)
		if count > 0 {
			emptyReads = 0
			if !runtime.InputSizeAllowed(data.Len() + count) {
				return nil, errInputTooLarge
			}
			data.Write(buffer[:count])
		} else if err == nil {
			emptyReads++
			if emptyReads >= 100 {
				return nil, io.ErrNoProgress
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return data.Bytes(), nil
			}
			return nil, err
		}
	}
}

func appendInteractiveLine(input *strings.Builder, line string) {
	input.WriteString(line)
}

func interactiveStatusError(status int) error {
	if status == 0 {
		return nil
	}
	return exitStatus(status)
}

func (c command) runScript(ctx context.Context, controller *interruptController, script string) error {
	return c.runScriptAs(ctx, controller, script, scriptInvocation{name: defaultScriptName})
}

type exitStatus int

func (e exitStatus) Error() string {
	return fmt.Sprintf("exit status %d", int(e))
}
