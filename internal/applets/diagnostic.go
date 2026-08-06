package applets

import (
	"errors"
	"fmt"
	"io/fs"
)

// Applet diagnostics name the operand exactly as the user wrote it. The
// resolved host path names one machine and Windows spells its own errors, so
// neither belongs in text a script greps or a behavior case compares byte for
// byte (docs/design/windows-execution-model.md, "Error Diagnostics").
//
// The shell prints "<applet>: <error>", so none of these carry the applet name.
//
// busybox uses two shapes and is not consistent about which, so neither are we:
// applets that open through open_or_warn (libbb/xfuncs_printf.c:169) say
// "can't open '<operand>'", and applets that go through fopen_or_warn ->
// bb_simple_perror_msg (libbb/wfopen.c:11) say only "<operand>".

type openError struct {
	operand string
	err     error
}

func cannotOpen(operand string, err error) error {
	return openError{operand: operand, err: err}
}

func (e openError) Error() string {
	return fmt.Sprintf("can't open '%s': %s", e.operand, causeText(e.err))
}

func (e openError) Unwrap() error { return e.err }

type operandError struct {
	operand string
	err     error
}

func operandFailure(operand string, err error) error {
	return operandError{operand: operand, err: err}
}

func (e operandError) Error() string {
	return fmt.Sprintf("%s: %s", e.operand, causeText(e.err))
}

func (e operandError) Unwrap() error { return e.err }

// causeText spells a cause the way strerror does. The fallback unwraps
// *fs.PathError rather than printing it, because its own Error() repeats the
// host path this whole file exists to keep out of diagnostics.
func causeText(err error) string {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "No such file or directory"
	case errors.Is(err, fs.ErrPermission):
		return "Permission denied"
	case errors.Is(err, fs.ErrExist):
		return "File exists"
	}
	if pathErr, ok := errors.AsType[*fs.PathError](err); ok {
		return pathErr.Err.Error()
	}
	return err.Error()
}
