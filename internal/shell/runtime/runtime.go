package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/xiongnemo/nemosh/internal/applets"
)

type Streams struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

type Runtime struct {
	registry applets.Registry
	streams  Streams
	vars     map[string]string
	traps    map[string]string
	params   *parameters
	readonly map[string]struct{}
}

func New(registry applets.Registry, streams Streams) Runtime {
	return Runtime{registry: registry, streams: fillStreams(streams), vars: map[string]string{}, traps: map[string]string{}, params: &parameters{}, readonly: map[string]struct{}{}}
}

func fillStreams(streams Streams) Streams {
	if streams.Stdin == nil {
		streams.Stdin = bytes.NewReader(nil)
	}
	if streams.Stdout == nil {
		streams.Stdout = io.Discard
	}
	if streams.Stderr == nil {
		streams.Stderr = io.Discard
	}
	return streams
}

type lineResult struct {
	status  int
	control flowControl
}

type flowControl int

const (
	flowNone flowControl = iota
	flowExit
	flowBreak
	flowContinue
)

func (r Runtime) runLine(ctx context.Context, line string) lineResult {
	segments := splitList(line)
	status := 0
	operator := ""
	for _, segment := range segments {
		if segment.operator != "" {
			operator = segment.operator
			continue
		}
		if operator == "&&" && status != 0 {
			continue
		}
		if operator == "||" && status == 0 {
			continue
		}
		args, err := splitWords(segment.text)
		if err != nil {
			fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
			return lineResult{status: 2}
		}
		args = r.expandArgs(ctx, args)
		if len(args) == 0 {
			continue
		}
		if len(args) == 1 && isAssignment(args[0]) {
			name, value, _ := strings.Cut(args[0], "=")
			status = r.assignVar(name, value)
			continue
		}
		if args[0] == "exit" {
			return lineResult{status: exitStatus(args[1:]), control: flowExit}
		}
		if args[0] == "break" {
			return lineResult{control: flowBreak}
		}
		if args[0] == "continue" {
			return lineResult{control: flowContinue}
		}
		status = r.runPipeline(ctx, args)
	}
	return lineResult{status: status}
}

func (r Runtime) runCommandWithRedirects(ctx context.Context, args []string) int {
	commandArgs, streams, cleanup, err := r.applyRedirects(args)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		return 1
	}
	status := (Runtime{registry: r.registry, streams: streams, vars: r.vars, traps: r.traps, params: r.params, readonly: r.readonly}).runCommand(ctx, commandArgs)
	if err := cleanup(); err != nil && status == 0 {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		return 1
	}
	return status
}

func (r Runtime) runCommand(ctx context.Context, args []string) int {
	switch args[0] {
	case ".":
		return r.dot(ctx, args[1:])
	case "cd":
		return r.cd(args[1:])
	case "command":
		return r.command(ctx, args[1:])
	case "eval":
		return r.eval(ctx, args[1:])
	case "export":
		return r.export(args[1:])
	case "unset":
		return r.unset(args[1:])
	case "pwd":
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(r.streams.Stderr, "pwd: %v\n", err)
			return 1
		}
		fmt.Fprintln(r.streams.Stdout, filepathDisplay(cwd))
		return 0
	case "read":
		return r.read(args[1:])
	case "readonly":
		return r.readonlyBuiltin(args[1:])
	case "set":
		return r.set(args[1:])
	case "shift":
		return r.shift(args[1:])
	case "trap":
		return r.trap(args[1:])
	}
	applet, ok := r.registry.Lookup(args[0])
	if !ok {
		return r.runExternal(ctx, args)
	}
	err := applet.Run(ctx, args[1:], r.streams.Stdin, r.streams.Stdout, r.streams.Stderr)
	if err == nil {
		return 0
	}
	if errors.Is(err, applets.ErrExitFalse) {
		return 1
	}
	fmt.Fprintf(r.streams.Stderr, "%s: %v\n", args[0], err)
	return 1
}

func (r Runtime) runExternal(ctx context.Context, args []string) int {
	cmd := exec.CommandContext(ctx, platformPath(args[0]), args[1:]...)
	cmd.Stdin = r.streams.Stdin
	cmd.Stdout = r.streams.Stdout
	cmd.Stderr = r.streams.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(r.streams.Stderr, "%s: not found\n", args[0])
		return 127
	}
	return 0
}

func (r Runtime) export(args []string) int {
	for _, arg := range args {
		name, value, hasValue := strings.Cut(arg, "=")
		if name == "" {
			return 2
		}
		if !hasValue {
			value = r.vars[name]
		} else if r.isReadonly(name) {
			fmt.Fprintf(r.streams.Stderr, "export: %s: readonly variable\n", name)
			return 1
		}
		r.vars[name] = value
		if err := os.Setenv(name, value); err != nil {
			fmt.Fprintf(r.streams.Stderr, "export: %s: %v\n", name, err)
			return 1
		}
	}
	return 0
}

func (r Runtime) unset(args []string) int {
	for _, name := range args {
		if r.isReadonly(name) {
			fmt.Fprintf(r.streams.Stderr, "unset: %s: readonly variable\n", name)
			return 1
		}
		delete(r.vars, name)
		if err := os.Unsetenv(name); err != nil {
			fmt.Fprintf(r.streams.Stderr, "unset: %s: %v\n", name, err)
			return 1
		}
	}
	return 0
}

func (r Runtime) cd(args []string) int {
	target := "."
	if len(args) > 0 {
		target = args[0]
	}
	if target == "//" || (strings.HasPrefix(target, "//") && strings.Count(strings.Trim(target, "/"), "/") == 0) {
		fmt.Fprintf(r.streams.Stderr, "cd: %s: No such file or directory\n", target)
		fmt.Fprintf(r.streams.Stderr, "hint: %s is not a directory root; use %s/share\n", target, strings.TrimRight(target, "/"))
		return 1
	}
	if err := os.Chdir(platformPath(target)); err != nil {
		fmt.Fprintf(r.streams.Stderr, "cd: %s: %v\n", target, err)
		return 1
	}
	return 0
}

func exitStatus(args []string) int {
	if len(args) == 0 {
		return 0
	}
	status, err := strconv.Atoi(args[0])
	if err != nil {
		return 2
	}
	return status & 0xff
}

func normalizeCRLF(script string) string {
	return strings.ReplaceAll(script, "\r\n", "\n")
}
