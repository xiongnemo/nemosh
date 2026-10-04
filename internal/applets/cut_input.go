package applets

import (
	"context"
	"errors"
	"io"
)

// busybox substitutes "-" for an absent operand list and then loops, holding
// retval at EXIT_FAILURE across an unreadable operand instead of stopping at it
// (coreutils/cut.c tail). sort is the deliberate opposite and says so in a
// comment: "coreutils 6.9 compat: abort on first open error" (sort.c:566).
func runCutInputs(ctx context.Context, view ProcessView, spec cutSpec, operands []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(operands) == 0 {
		operands = []string{"-"}
	}
	// Buffered, and flushed when the input runs dry or something is said; see
	// filter_output.go.
	output := newFilterOutput(stdout)
	var failed error
	for _, operand := range operands {
		err := cutOneInput(ctx, view, &spec, operand, stdin, output)
		if err == nil {
			continue
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			_ = output.Flush()
			return err
		}
		_ = output.Flush()
		failed = writeCutDiagnostic(stderr, inputDiagnostic("cut", err))
	}
	if err := output.Flush(); err != nil {
		return err
	}
	return failed
}

func cutOneInput(ctx context.Context, view ProcessView, spec *cutSpec, operand string, stdin io.Reader, output filterOutput) error {
	if operand == "-" {
		return spec.cutFile(output.input(stdin), output)
	}
	input, err := OpenProcessInput(ctx, view, operand)
	if err != nil {
		return inputFailure(operand, err)
	}
	readErr := spec.cutFile(output.input(input), output)
	closeErr := input.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return inputFailure(operand, err)
	}
	return nil
}
