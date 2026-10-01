package applets

import (
	"context"
	"errors"
	"io"
	"time"
)

// pipe_progress copies its input to its output and shows on stderr that it is moving, as
// busybox's does (debianutils/pipe_progress.c): a dot each time a read comes in a later second
// than the one before, and a newline when the input ends. It reads 4096 bytes at a time, as
// busybox does, so a slow pipe gets about a dot a second. Arguments are ignored, as there.
//
// The name is passed in, as `[`'s is, because the registry's names are read from its source
// too (internal/appletmanifest), and no constructor's name spells an underscore.
func newPipeProgressApplet(name string) Applet {
	return simpleApplet{name: name, runContext: func(ctx context.Context, _ []string, stdin io.Reader, stdout, stderr io.Writer) error {
		reader := contextReader{ctx: ctx, reader: stdin}
		buffer := make([]byte, 4096)
		last := pipeProgressNow()
		for {
			count, err := reader.Read(buffer)
			if count > 0 {
				if now := pipeProgressNow(); now != last {
					last = now
					_, _ = io.WriteString(stderr, ".")
				}
				if _, writeErr := stdout.Write(buffer[:count]); writeErr != nil {
					_, _ = io.WriteString(stderr, "\n")
					return writeErr
				}
			}
			if err != nil {
				_, _ = io.WriteString(stderr, "\n")
				if errors.Is(err, io.EOF) {
					return nil
				}
				return err
			}
		}
	}}
}

// pipeProgressNow is the second a read came in, which a test moves by hand.
var pipeProgressNow = func() int64 { return time.Now().Unix() }
