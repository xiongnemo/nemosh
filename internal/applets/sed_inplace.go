package applets

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
)

// -i, editing each file in place.
//
// This is the option that needed a decision rather than an implementation, and
// the decision turned out to be smaller than it looked. The concern was that
// rewriting a file forces a choice of *output* encoding -- the same choice
// deferred for sed's UTF-16 reading. It does not: sed here is byte-exact, so the
// bytes written back are the bytes read, transformed. The encoding question only
// arrives if sed ever starts decoding UTF-16 on input, and it is still deferred
// until then. See docs/support-matrix.md, Text encodings.

// runSedInPlace edits every operand, one at a time.
//
// Each file is its own stream: line numbers restart and `$` is that file's last
// line. That is GNU's behaviour and the coherent reading of what -i means -- an
// independent edit per file. busybox restarts the numbering but leaves an open
// address *range* running across the boundary, so `sed -i -n '2,3p' a b` prints
// b's first line because a's range never closed. Measured 2026-08-22; that is
// state reuse rather than a rule, and the divergence is recorded.
func runSedInPlace(ctx context.Context, program *sedProgram, operands []string, suffix string, stderr io.Writer) error {
	if len(operands) == 0 {
		// There is nothing to edit in place when the input is a stream. GNU says
		// exactly this.
		return fmt.Errorf("no input files")
	}
	view := ProcessViewFromContext(ctx)
	failed := false
	for _, operand := range operands {
		native, err := resolveHostPath(view, operand)
		if err != nil {
			fmt.Fprintf(stderr, "sed: %v\n", operandFailure(operand, err))
			failed = true
			continue
		}
		// Reset between files, so a range left open by one does not select lines
		// in the next.
		program.reset()
		if err := editSedFileInPlace(program, operand, native, suffix); err != nil {
			fmt.Fprintf(stderr, "sed: %v\n", err)
			failed = true
			continue
		}
	}
	if failed {
		return ExitStatus(1)
	}
	return nil
}

// editSedFileInPlace transforms one file and writes it back.
//
// The whole result is built before anything is written, because the output is
// derived from the input: streaming it into the same path would overwrite lines
// still waiting to be read. That is also what makes a failure safe -- a script
// that fails halfway leaves the file as it was.
func editSedFileInPlace(program *sedProgram, operand, native, suffix string) error {
	original, err := os.ReadFile(native)
	if err != nil {
		return operandFailure(operand, err)
	}
	var transformed bytes.Buffer
	stream := &sedStream{
		openers:     []func() (io.ReadCloser, error){func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(original)), nil }},
		onOpenError: func(error) {},
	}
	if err := program.execute(stream, &transformed); err != nil {
		return operandFailure(operand, err)
	}
	info, err := os.Stat(native)
	mode := os.FileMode(0o644)
	if err == nil {
		mode = info.Mode().Perm()
	}
	if suffix != "" {
		// The original is renamed to the backup rather than copied, so the old
		// bytes are never rewritten and a large file costs nothing to keep.
		if err := os.Rename(native, native+suffix); err != nil {
			return operandFailure(operand, err)
		}
	}
	if err := os.WriteFile(native, transformed.Bytes(), mode); err != nil {
		return operandFailure(operand, err)
	}
	return nil
}

// reset clears the per-run state an address carries, so a program can be applied
// to a second file as though it had just been parsed.
//
// Ranges are the only mutable state: `active` records that an opening address has
// matched and the closing one has not. Blocks are walked too, since an address
// inside one is state just the same.
func (p *sedProgram) reset() { resetSedCommands(p.commands) }

func resetSedCommands(commands []*sedCommand) {
	for _, command := range commands {
		command.address.active = false
		resetSedCommands(command.block)
	}
}
