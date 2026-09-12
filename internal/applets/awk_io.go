package applets

import (
	"bytes"
	"io"
	"os"
	"sort"
)

// Where a `print` can send its output, and what `close` and `fflush` do about it.
//
// **A destination is opened once and kept**, keyed by the text of its name. That is the
// rule that makes the commonest redirect work: `{ print > "out" }` over a hundred records
// truncates the file once and appends the other ninety-nine times, because the second
// `print` finds the stream the first one opened. Re-opening per record would leave one line
// in the file and is the bug this shape cannot have.
//
// A **pipe collects and runs at close.** Its applet is handed everything the program wrote,
// in one go, when `close()` is called or the program ends. Running it concurrently would be
// the other option, and it is rejected here: `print | "sort"` cannot produce anything until
// its input ends anyway, and a buffered pipe makes the output order deterministic rather
// than a race. It also matches busybox, which prints `direct` before the piped `x` where
// gawk prints them the other way round.

// awkOutput is one destination the program is writing to.
type awkOutput struct {
	writer io.Writer
	file   *os.File
	// buffer and command are set for a pipe: what has been written, and the applet that
	// will be given it.
	buffer  *bytes.Buffer
	command []string
}

// resolveOutput answers the writer for a redirect, opening it on first use.
func (in *awkInterp) resolveOutput(name, operator string) (io.Writer, error) {
	if existing, open := in.outputs[name]; open {
		return existing.writer, nil
	}
	stream, err := in.openOutput(name, operator)
	if err != nil {
		return nil, err
	}
	if in.outputs == nil {
		in.outputs = map[string]*awkOutput{}
	}
	in.outputs[name] = stream
	return stream.writer, nil
}

func (in *awkInterp) openOutput(name, operator string) (*awkOutput, error) {
	// The two standard streams are reachable by name, which both references allow and
	// which is how a program writes a diagnostic without a shell to redirect it.
	switch {
	case name == "/dev/stdout" || name == "-":
		return &awkOutput{writer: in.output}, nil
	case name == "/dev/stderr":
		return &awkOutput{writer: in.errors}, nil
	}
	if operator == "|" {
		command, err := awkCommandWords(name)
		if err != nil {
			return nil, err
		}
		buffer := &bytes.Buffer{}
		return &awkOutput{writer: buffer, buffer: buffer, command: command}, nil
	}
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if operator == ">>" {
		flags = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	}
	file, err := os.OpenFile(name, flags, 0o644)
	if err != nil {
		return nil, cannotCreate(name, err)
	}
	return &awkOutput{writer: file, file: file}, nil
}

// writeTo sends a print's text to its redirect, or to standard output when it has none.
func (in *awkInterp) writeTo(redirect *awkRedirect, text string) error {
	if redirect == nil {
		return in.write(text)
	}
	target, err := in.eval(redirect.target)
	if err != nil {
		return err
	}
	writer, err := in.resolveOutput(target.str(in.convfmt()), redirect.operator)
	if err != nil {
		return err
	}
	_, err = io.WriteString(writer, text)
	return err
}

// closeStream is `close(name)`.
//
// **-1 when there was nothing open by that name**, which is what a program tests, and 0
// when there was. Both references agree, and neither treats closing an unopened name as an
// error.
func (in *awkInterp) closeStream(name string) int {
	closed := false
	if stream, open := in.outputs[name]; open {
		delete(in.outputs, name)
		if err := in.finishOutput(stream); err != nil {
			// A failure to flush or to run the pipe is reported where it happened
			// rather than swallowed, but `close` still answers a number.
			in.reportLater(err)
		}
		closed = true
	}
	if stream, open := in.inputs[name]; open {
		delete(in.inputs, name)
		stream.close()
		closed = true
	}
	if !closed {
		return -1
	}
	return 0
}

// finishOutput flushes a destination and, if it is a pipe, runs its applet.
func (in *awkInterp) finishOutput(stream *awkOutput) error {
	if stream.command != nil {
		return in.runOutputPipe(stream)
	}
	if stream.file != nil {
		return stream.file.Close()
	}
	return nil
}

// flushOutputs is `fflush`, and also what runs before anything that must see the output in
// order -- a pipe's applet, or `system`.
func (in *awkInterp) flushOutputs(name string) int {
	if name != "" {
		if _, open := in.outputs[name]; !open {
			return -1
		}
	}
	_ = in.buffered.Flush()
	for _, stream := range in.outputs {
		if stream.file != nil {
			_ = stream.file.Sync()
		}
	}
	return 0
}

// closeAllStreams finishes everything still open when the program ends.
//
// In name order, so that two pipes left open at the end produce their output in an order
// that does not depend on a map's iteration.
func (in *awkInterp) closeAllStreams() error {
	names := make([]string, 0, len(in.outputs))
	for name := range in.outputs {
		names = append(names, name)
	}
	sort.Strings(names)
	var first error
	for _, name := range names {
		stream := in.outputs[name]
		delete(in.outputs, name)
		if err := in.finishOutput(stream); err != nil && first == nil {
			first = err
		}
	}
	for name, stream := range in.inputs {
		delete(in.inputs, name)
		stream.close()
	}
	return first
}

// reportLater records a failure that happened where no error could be returned.
func (in *awkInterp) reportLater(err error) {
	if in.deferred == nil {
		in.deferred = err
	}
}
