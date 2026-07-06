package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func main() {
	cmd := command{stdin: os.Stdin, stdout: os.Stdout, stderr: os.Stderr}
	if err := cmd.run(context.Background(), os.Args); err != nil {
		if errors.Is(err, applets.ErrExitFalse) {
			os.Exit(1)
		}
		var status exitStatus
		if errors.As(err, &status) {
			os.Exit(int(status))
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type command struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

func run(ctx context.Context, args []string) error {
	cmd := command{stdin: os.Stdin, stdout: os.Stdout, stderr: os.Stderr}
	return cmd.run(ctx, args)
}

func (c command) run(ctx context.Context, args []string) error {
	name := applets.InvocationName(args)
	if applet, ok := applets.DefaultRegistry.Lookup(name); ok {
		return applet.Run(ctx, args[1:], c.stdin, c.stdout, c.stderr)
	}

	if len(args) > 2 && args[1] == "-c" {
		return c.runScript(ctx, args[2])
	}
	if len(args) > 1 && args[1] == "-i" {
		return c.runInteractive(ctx)
	}

	if len(args) > 1 {
		if applet, ok := applets.DefaultRegistry.Lookup(args[1]); ok {
			return applet.Run(ctx, args[2:], c.stdin, c.stdout, c.stderr)
		}
	}

	data, err := io.ReadAll(c.stdin)
	if err != nil {
		return fmt.Errorf("nemosh: read stdin: %w", err)
	}
	return c.runScript(ctx, string(data))
}

func (c command) runInteractive(ctx context.Context) error {
	scanner := bufio.NewScanner(c.stdin)
	for {
		fmt.Fprint(c.stdout, "$ ")
		if !scanner.Scan() {
			return scanner.Err()
		}
		line := scanner.Text()
		if strings.TrimSpace(line) == "exit" || strings.HasPrefix(strings.TrimSpace(line), "exit ") {
			return c.runScript(ctx, line)
		}
		if err := c.runScript(ctx, line); err != nil {
			var status exitStatus
			if errors.As(err, &status) && status == 0 {
				return nil
			}
			return err
		}
	}
}

func (c command) runScript(ctx context.Context, script string) error {
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdin: c.stdin, Stdout: c.stdout, Stderr: c.stderr})
	status := rt.RunScript(ctx, script)
	if status == 0 {
		return nil
	}
	return exitStatus(status)
}

type exitStatus int

func (e exitStatus) Error() string {
	return fmt.Sprintf("exit status %d", int(e))
}
