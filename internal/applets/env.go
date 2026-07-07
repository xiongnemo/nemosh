package applets

import (
	"context"
	"fmt"
	"io"
	"os"
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

func newEnvApplet() Applet {
	return envApplet{}
}

func (envApplet) Name() string {
	return "env"
}

func (envApplet) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	invocation, err := parseEnvInvocation(args)
	if err != nil {
		return err
	}
	return withEnvScope(invocation, func() error {
		if len(invocation.command) == 0 {
			return printEnvironment(stdout)
		}
		applet, ok := DefaultRegistry.Lookup(invocation.command[0])
		if !ok {
			return fmt.Errorf("%s: not found", invocation.command[0])
		}
		return applet.Run(ctx, invocation.command[1:], stdin, stdout, stderr)
	})
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
		invocation.assignments = append(invocation.assignments, envAssignment{name: name, value: value})
		remaining = remaining[1:]
	}
	invocation.command = remaining
	return invocation, nil
}

func withEnvScope(invocation envInvocation, run func() error) error {
	original := os.Environ()
	restore := func() error { return restoreEnv(original) }
	if invocation.ignoreEnvironment {
		os.Clearenv()
	}
	for _, assignment := range invocation.assignments {
		if err := os.Setenv(assignment.name, assignment.value); err != nil {
			if restoreErr := restore(); restoreErr != nil {
				return fmt.Errorf("env: %s: %w; restore environment: %w", assignment.name, err, restoreErr)
			}
			return fmt.Errorf("env: %s: %w", assignment.name, err)
		}
	}
	err := run()
	restoreErr := restore()
	if err != nil && restoreErr != nil {
		return fmt.Errorf("env: %w; restore environment: %w", err, restoreErr)
	}
	if err != nil {
		return err
	}
	return restoreErr
}

func restoreEnv(items []string) error {
	os.Clearenv()
	for _, item := range items {
		name, value, _ := strings.Cut(item, "=")
		if err := os.Setenv(name, value); err != nil {
			return err
		}
	}
	return nil
}

func printEnvironment(stdout io.Writer) error {
	for _, item := range os.Environ() {
		if _, err := fmt.Fprintln(stdout, item); err != nil {
			return err
		}
	}
	return nil
}

func newPrintenvApplet() Applet {
	return simpleApplet{name: "printenv", run: func(args []string, _ io.Reader, stdout, _ io.Writer) error {
		if len(args) == 0 {
			for _, item := range os.Environ() {
				fmt.Fprintln(stdout, item)
			}
			return nil
		}
		status := error(nil)
		for _, name := range args {
			value, ok := os.LookupEnv(name)
			if !ok {
				status = ErrExitFalse
				continue
			}
			fmt.Fprintln(stdout, value)
		}
		return status
	}}
}
