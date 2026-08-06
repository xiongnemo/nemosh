package applets

import (
	"context"
	"fmt"
	"io"
	"strings"
)

type envApplet struct{}

type envAssignment struct {
	name  string
	value string
}

type envInvocation struct {
	ignoreEnvironment bool
	assignments       []envAssignment
	command           []string
}

func newEnvApplet() Applet     { return envApplet{} }
func (envApplet) Name() string { return "env" }

func (envApplet) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	invocation, err := parseEnvInvocation(args)
	if err != nil {
		return err
	}
	view := deriveProcessView(ProcessViewFromContext(ctx), invocation)
	ctx = WithProcessView(ctx, view)
	if len(invocation.command) == 0 {
		return printEnvironment(stdout, view.Environ())
	}
	applet, ok := DefaultRegistry.Lookup(invocation.command[0])
	if !ok {
		return commandNotFound(invocation.command[0])
	}
	return applet.Run(ctx, invocation.command[1:], stdin, stdout, stderr)
}

func parseEnvInvocation(args []string) (envInvocation, error) {
	invocation := envInvocation{}
	remaining := args
	if len(remaining) > 0 && strings.HasPrefix(remaining[0], "-") {
		if remaining[0] != "-i" {
			return invocation, fmt.Errorf("unsupported env option: %s", remaining[0])
		}
		invocation.ignoreEnvironment = true
		remaining = remaining[1:]
	}
	for len(remaining) > 0 && strings.Contains(remaining[0], "=") {
		name, value, _ := strings.Cut(remaining[0], "=")
		if name == "" {
			return invocation, fmt.Errorf("invalid variable name: empty")
		}
		invocation.assignments = append(invocation.assignments, envAssignment{name: name, value: value})
		remaining = remaining[1:]
	}
	invocation.command = remaining
	return invocation, nil
}

func deriveProcessView(parent ProcessView, invocation envInvocation) staticProcessView {
	items := parent.Environ()
	if invocation.ignoreEnvironment {
		items = nil
	}
	view := newStaticProcessView(parent, items)
	for _, assignment := range invocation.assignments {
		view.set(assignment.name, assignment.value)
	}
	return view
}

func printEnvironment(stdout io.Writer, items []string) error {
	for _, item := range items {
		if _, err := fmt.Fprintln(stdout, item); err != nil {
			return err
		}
	}
	return nil
}

func newPrintenvApplet() Applet {
	return printenvApplet{}
}

type printenvApplet struct{}

func (printenvApplet) Name() string { return "printenv" }
func (printenvApplet) Run(ctx context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
	view := ProcessViewFromContext(ctx)
	if len(args) == 0 {
		return printEnvironment(stdout, view.Environ())
	}
	var status error
	for _, name := range args {
		value, ok := view.LookupEnv(name)
		if !ok {
			status = ErrExitFalse
			continue
		}
		fmt.Fprintln(stdout, value)
	}
	return status
}
