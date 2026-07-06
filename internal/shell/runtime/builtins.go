package runtime

import (
	"fmt"
	"io"
	"strings"
)

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
