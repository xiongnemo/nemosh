package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
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
			return cutReadError{path: operand, err: err}
		}
		readErr := cutReader(input, stdout, options)
		closeErr := input.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			return cutReadError{path: operand, err: err}
		}
	}
	return nil
}

type cutReadError struct {
	path string
	err  error
}

func (e cutReadError) Error() string { return e.err.Error() }
func (e cutReadError) Unwrap() error { return e.err }

func cutInputError(err error) string {
	path := ""
	if readErr, ok := errors.AsType[cutReadError](err); ok {
		path = readErr.path
	}
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Sprintf("cut: %s: No such file or directory", path)
	}
	return fmt.Sprintf("cut: %s: %v", path, err)
}
