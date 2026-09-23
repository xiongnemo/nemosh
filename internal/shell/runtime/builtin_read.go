package runtime

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
)

type contextReader interface {
	ReadContext(context.Context, []byte) (int, error)
}

// read is POSIX `read`, plus the bash options a script written this decade
// expects. See builtin_read_options.go for what it had been doing instead.
//
// Status, measured against bash: 0 when the delimiter was reached, 1 at end of
// input -- and the names are still assigned in that case, because
// `printf a | read x` leaves x holding `a` and reports failure. A timeout is
// 128 + SIGALRM, which is 142, the number a script tests for.
func (r Runtime) read(ctx context.Context, args []string) int {
	options, err := parseReadOptions(args)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "read: %v\n", err)
		return 2
	}
	input, err := r.fds.reader(options.descriptor)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "read: %v\n", err)
		return 1
	}
	r.writeReadPrompt(options)
	if line, handled, err := r.readSilently(ctx, input, options); handled {
		if err != nil {
			if ctx.Err() != nil {
				return contextStatus(ctx)
			}
			fmt.Fprintf(r.streams.Stderr, "read: %v\n", err)
			return 1
		}
		return r.assignReadResult(options, line)
	}
	line, status := r.collectWithTimeout(ctx, input, options)
	if status != 0 {
		return status
	}
	if ctx.Err() != nil {
		return contextStatus(ctx)
	}
	if assigned := r.assignReadResult(options, line); assigned != 0 {
		return assigned
	}
	if !line.delimited {
		return 1
	}
	return 0
}

// writeReadPrompt puts -p on the terminal.
//
// To stderr, so that `read -p 'name: ' v` inside `$(...)` does not put the prompt
// into the value being captured -- bash writes it to stderr for the same reason.
func (r Runtime) writeReadPrompt(options readOptions) {
	if options.prompt == "" {
		return
	}
	fmt.Fprint(r.streams.Stderr, options.prompt)
}

// collectWithTimeout reads the line, giving up after -t.
//
// The read runs on its own goroutine because the reader may not be cancellable --
// readWithContext says so itself. On a timeout the goroutine is abandoned rather
// than waited for, which is the same trade the comment there describes: a blocked
// read on an arbitrary reader cannot be taken back, and holding the shell until it
// returns would make `read -t` the one thing it must not be, a wait with no upper
// bound.
func (r Runtime) collectWithTimeout(ctx context.Context, input io.Reader, options readOptions) (readLineResult, int) {
	if !options.hasTimeout {
		line, err := collectReadLine(ctx, input, options)
		if err != nil && ctx.Err() == nil {
			fmt.Fprintf(r.streams.Stderr, "read: %v\n", err)
			return readLineResult{}, 1
		}
		return line, 0
	}
	type outcome struct {
		line readLineResult
		err  error
	}
	results := make(chan outcome, 1)
	readCtx, cancelRead := context.WithCancel(ctx)
	defer cancelRead()
	go func() {
		line, err := collectReadLine(readCtx, input, options)
		results <- outcome{line: line, err: err}
	}()
	timer := time.NewTimer(options.timeout)
	defer timer.Stop()
	select {
	case result := <-results:
		if result.err != nil && ctx.Err() == nil {
			fmt.Fprintf(r.streams.Stderr, "read: %v\n", result.err)
			return readLineResult{}, 1
		}
		return result.line, 0
	case <-timer.C:
		return readLineResult{}, 142
	case <-ctx.Done():
		return readLineResult{}, contextStatus(ctx)
	}
}

// assignReadResult distributes the line: to an array for -a, and to the names
// otherwise.
//
// With no names at all the whole line goes to REPLY *unmodified* -- no field
// splitting and no trimming. bash's documentation spells that out and it was
// measured: `read` over `  a  b  ` leaves REPLY holding both runs of blanks.
func (r Runtime) assignReadResult(options readOptions, line readLineResult) int {
	separators := r.fieldSeparators()
	if options.arrayName != "" {
		if r.isReadonly(options.arrayName) {
			fmt.Fprintf(r.streams.Stderr, "%s: readonly variable\n", options.arrayName)
			return 1
		}
		fields := splitReadFields(line.text, line.escaped, separators, 0)
		r.arrays.set(options.arrayName, fields)
		r.syncArrayScalar(options.arrayName)
		return 0
	}
	// A readonly name is refused before anything is assigned, and it is not a shell error:
	// busybox's read reports it and the script goes on, which is the one place busybox
	// does not abort over a readonly variable. Status 2, as busybox answers.
	targets := options.names
	if len(targets) == 0 {
		targets = []string{"REPLY"}
	}
	for _, name := range targets {
		if r.isReadonly(name) {
			fmt.Fprintf(r.streams.Stderr, "read: %s: readonly variable\n", name)
			return 2
		}
	}
	if len(options.names) == 0 {
		return r.assignVar("REPLY", line.text)
	}
	fields := splitReadFields(line.text, line.escaped, separators, len(options.names))
	for index, name := range options.names {
		value := ""
		if index < len(fields) {
			value = fields[index]
		}
		if status := r.assignVar(name, value); status != 0 {
			return status
		}
	}
	return 0
}

func readWithContext(ctx context.Context, input io.Reader, buffer []byte) (int, error) {
	if reader, ok := input.(contextReader); ok {
		return reader.ReadContext(ctx, buffer)
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	// This is where an applet's stdin actually ends up inside the shell: contextReader in
	// internal/applets forwards to descriptorReader.ReadContext, which forwards to here, and
	// only here is the underlying file in hand. So a file is read through the one
	// implementation that can end a read already blocked -- there used to be a second copy of
	// it here, and it carried the same data race as the first. See applets.ReadInterruptibly.
	//
	// An arbitrary blocking reader still cannot be cancelled without abandoning a read
	// goroutine, which would go on to eat a keystroke, so anything else is read plainly.
	if file, isFile := input.(*os.File); isFile {
		return applets.ReadInterruptibly(ctx, file, buffer)
	}
	return input.Read(buffer)
}
