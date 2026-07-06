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
}

func New(registry applets.Registry, streams Streams) Runtime {
	return Runtime{registry: registry, streams: fillStreams(streams)}
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
		args, err := splitWords(line)
		if err != nil {
			fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
			return 2
		}
		if len(args) == 0 {
			continue
		}
		if args[0] == "exit" {
			return exitStatus(args[1:])
		}
		status = r.runCommand(ctx, args)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		return 2
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

func splitWords(line string) ([]string, error) {
	var args []string
	var current strings.Builder
	inSingle := false
	inDouble := false
	escaped := false
	flush := func() {
		if current.Len() > 0 {
			args = append(args, current.String())
			current.Reset()
		}
	}
	for _, r := range line {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		switch {
		case r == '\\' && !inSingle:
			escaped = true
		case r == '\'' && !inDouble:
			inSingle = !inSingle
		case r == '"' && !inSingle:
			inDouble = !inDouble
		case (r == ' ' || r == '\t') && !inSingle && !inDouble:
			flush()
		default:
			current.WriteRune(r)
		}
	}
	if escaped {
		current.WriteRune('\\')
	}
	if inSingle || inDouble {
		return nil, errors.New("unterminated quote")
	}
	flush()
	return args, nil
}

func platformPath(path string) string {
	if len(path) >= 3 && path[0] == '/' && path[2] == '/' && isDriveLetter(rune(path[1])) {
		return string(path[1]) + ":/" + path[3:]
	}
	return path
}

func filepathDisplay(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	if len(path) >= 2 && path[1] == ':' && isDriveLetter(rune(path[0])) {
		return "/" + strings.ToLower(path[:1]) + path[2:]
	}
	return path
}

func isDriveLetter(r rune) bool {
	return ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z')
}
