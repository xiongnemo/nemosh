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
	// unset is each -u NAME, in order; one holding `=` sets instead, as busybox's putenv does.
	unset []string
	// null is -0: the environment printed with a NUL after each entry.
	null        bool
	assignments []envAssignment
	command     []string
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
		if invocation.null {
			for _, item := range view.Environ() {
				if _, err := fmt.Fprintf(stdout, "%s\x00", item); err != nil {
					return err
				}
			}
			return nil
		}
		return printEnvironment(stdout, view.Environ())
	}
	applet, ok := DefaultRegistry.Lookup(invocation.command[0])
	if !ok {
		return commandNotFound(invocation.command[0])
	}
	return applet.Run(ctx, invocation.command[1:], stdin, stdout, stderr)
}

// parseEnvInvocation reads busybox's `env [-i0] [-u NAME]... [-] [NAME=VALUE]... [PROG ARGS]`
// (coreutils/env.c), its options in order up to the first word that is none, and
// --ignore-environment, --null and --unset for their letters. A lone `-` is -i. It took -i
// alone and refused -u and -0.
func parseEnvInvocation(args []string) (envInvocation, error) {
	words := longOptionWords(args, map[string]string{"ignore-environment": "i", "null": "0", "unset": "u"})
	options, remaining, err := parseAppletOptionsInOrder(words, "i0", "u")
	if err != nil {
		return envInvocation{}, err
	}
	invocation := envInvocation{ignoreEnvironment: options.has('i'), null: options.has('0'), unset: options.all('u')}
	if len(remaining) > 0 && remaining[0] == "-" {
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
	for _, name := range invocation.unset {
		if key, value, found := strings.Cut(name, "="); found {
			view.set(key, value)
			continue
		}
		view.unset(name)
	}
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

// unset removes name, spelled as the environment compares names: in any case on Windows.
func (v staticProcessView) unset(name string) {
	for key := range v.values {
		if environmentNamesEqual(key, name) {
			delete(v.values, key)
		}
	}
}
