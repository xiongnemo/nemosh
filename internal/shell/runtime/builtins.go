package runtime

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
)

func (r Runtime) dot(ctx context.Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(r.streams.Stderr, ".: missing file")
		return 2
	}
	data, err := os.ReadFile(platformPath(args[0]))
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, ".: %s: %v\n", args[0], err)
		return 1
	}
	return r.runScript(ctx, string(data), false)
}

func (r Runtime) eval(ctx context.Context, args []string) int {
	if len(args) == 0 {
		return 0
	}
	return r.runScript(ctx, strings.Join(args, " "), false)
}

func (r Runtime) read(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(r.streams.Stderr, "read: missing variable name")
		return 2
	}
	line, err := readLine(r.streams.Stdin)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "read: %v\n", err)
		return 1
	}
	if line == "" {
		return 1
	}
	r.vars[args[0]] = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	return 0
}

func (r Runtime) trap(args []string) int {
	if len(args) != 2 || args[1] != "EXIT" {
		fmt.Fprintln(r.streams.Stderr, "trap: expected: trap command EXIT")
		return 2
	}
	r.traps["EXIT"] = args[0]
	return 0
}

func readLine(input io.Reader) (string, error) {
	var b strings.Builder
	buf := []byte{0}
	for {
		n, err := input.Read(buf)
		if n > 0 {
			b.WriteByte(buf[0])
			if buf[0] == '\n' {
				return b.String(), nil
			}
		}
		if err == io.EOF {
			return b.String(), nil
		}
		if err != nil {
			return "", err
		}
	}
}
