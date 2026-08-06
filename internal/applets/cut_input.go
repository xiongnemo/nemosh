package applets

import (
	"context"
	"errors"
	"io"
)

func runCutInputs(ctx context.Context, view ProcessView, options cutOptions, stdin io.Reader, stdout io.Writer) error {
	if len(options.operands) == 0 {
		return cutReader(stdin, stdout, options)
	}
	for _, operand := range options.operands {
		if operand == "-" {
			if err := cutReader(stdin, stdout, options); err != nil {
				return err
			}
			continue
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
	}
	return nil
}
