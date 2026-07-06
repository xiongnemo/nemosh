package runtime

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
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
}

func New(registry applets.Registry, streams Streams) Runtime {
	return Runtime{registry: registry, streams: fillStreams(streams), vars: map[string]string{}}
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

func (r Runtime) RunScript(ctx context.Context, script string) int {
	status := 0
	scanner := bufio.NewScanner(strings.NewReader(normalizeCRLF(script)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		result := r.runLine(ctx, line)
		status = result.status
		if result.stop {
			return status
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		return 2
	}
	return status
}

type lineResult struct {
	status int
	stop   bool
}

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
		args = r.expandArgs(args)
		if len(args) == 0 {
			continue
		}
		if len(args) == 1 && isAssignment(args[0]) {
			name, value, _ := strings.Cut(args[0], "=")
			r.vars[name] = value
			status = 0
			continue
		}
		if args[0] == "exit" {
			return lineResult{status: exitStatus(args[1:]), stop: true}
		}
		status = r.runCommandWithRedirects(ctx, args)
	}
	return lineResult{status: status}
}

func (r Runtime) runCommandWithRedirects(ctx context.Context, args []string) int {
	commandArgs, streams, cleanup, err := r.applyRedirects(args)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		return 1
	}
	status := (Runtime{registry: r.registry, streams: streams, vars: r.vars}).runCommand(ctx, commandArgs)
	if err := cleanup(); err != nil && status == 0 {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		return 1
	}
	return status
}

func (r Runtime) runCommand(ctx context.Context, args []string) int {
	switch args[0] {
	case "cd":
		return r.cd(args[1:])
	case "pwd":
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(r.streams.Stderr, "pwd: %v\n", err)
			return 1
		}
		fmt.Fprintln(r.streams.Stdout, filepathDisplay(cwd))
		return 0
	}
	applet, ok := r.registry.Lookup(args[0])
	if !ok {
		fmt.Fprintf(r.streams.Stderr, "%s: not found\n", args[0])
		return 127
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
