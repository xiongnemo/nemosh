package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

type contextReader interface {
	ReadContext(context.Context, []byte) (int, error)
}

func (r Runtime) read(ctx context.Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(r.streams.Stderr, "read: missing variable name")
		return 2
	}
	input, err := r.fds.reader(0)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "read: %v\n", err)
		return 1
	}
	line, err := readLine(ctx, input)
	if ctx.Err() != nil {
		return contextStatus(ctx)
	}
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "read: %v\n", err)
		return 1
	}
	if line == "" {
		return 1
	}
	value := strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	return r.assignVar(args[0], value)
}

func readLine(ctx context.Context, input io.Reader) (string, error) {
	var line strings.Builder
	buffer := []byte{0}
	for {
		count, err := readWithContext(ctx, input, buffer)
		if count > 0 {
			line.WriteByte(buffer[0])
			if buffer[0] == '\n' {
				return line.String(), nil
			}
		}
		if errors.Is(err, io.EOF) {
			return line.String(), nil
		}
		if err != nil {
			return "", err
		}
	}
}

func readWithContext(ctx context.Context, input io.Reader, buffer []byte) (int, error) {
	if reader, ok := input.(contextReader); ok {
		return reader.ReadContext(ctx, buffer)
	}
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
		// Arbitrary blocking readers cannot be canceled without a competing,
		// potentially abandoned read goroutine. The guarantee is intentionally
		// limited to contextReader implementations and platform file adapters.
		return input.Read(buffer)
	}
}
