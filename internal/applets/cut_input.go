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
func runCutInputs(ctx context.Context, view ProcessView, options cutOptions, stdin io.Reader, stdout, stderr io.Writer) error {
	operands := options.operands
	if len(operands) == 0 {
		operands = []string{"-"}
	}
	var failed error
	for _, operand := range operands {
		err := cutOneInput(ctx, view, options, operand, stdin, stdout)
		if err == nil {
			continue
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		failed = writeCutDiagnostic(stderr, inputDiagnostic("cut", err))
	}
	return failed
}

func cutOneInput(ctx context.Context, view ProcessView, options cutOptions, operand string, stdin io.Reader, stdout io.Writer) error {
	if operand == "-" {
		return cutReader(stdin, stdout, options)
	}
	input, err := OpenProcessInput(ctx, view, operand)
	if err != nil {
		return inputFailure(operand, err)
	}
	readErr := cutReader(input, stdout, options)
	closeErr := input.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return inputFailure(operand, err)
	}
	return nil
}
