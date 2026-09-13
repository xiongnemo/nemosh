package runtime

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"
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
	file, isFile := input.(*os.File)
	if !isFile || ctx.Done() == nil {
		// An arbitrary blocking reader still cannot be cancelled without abandoning a
		// read goroutine, which would go on to eat a keystroke. A file can be, below.
		return input.Read(buffer)
	}
	// This is where an applet's stdin actually ends up inside the shell: contextReader in
	// internal/applets forwards to descriptorReader.ReadContext, which forwards to here,
	// and only here is the underlying file in hand. Checking the context before the read
	// and never again is no use to an applet already sitting in one -- a console read
	// returns when a line arrives and not before, which is the whole of why Ctrl-C inside
	// `bc` did nothing until the next keystroke, and then spent that keystroke noticing.
	//
	// interruptPipeIO is CancelIoEx; the name is from its first caller. Measured against a
	// real console: a read blocked for 500ms, the call answered success, and the read
	// returned at once.
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			_ = interruptPipeIO(file)
		case <-stop:
		}
	}()
	read, err := file.Read(buffer)
	if read == 0 && ctx.Err() != nil {
		// An aborted console read reports EOF, which is what the input really ending also
		// reports -- so the context decides, not the error. Bytes that did arrive are
		// handed back; the read after them reports the cancellation.
		return 0, ctx.Err()
	}
	return read, err
}
