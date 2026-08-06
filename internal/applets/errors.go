package applets

import (
	"errors"
	"fmt"
)

var ErrExitFalse = errors.New("false")

type statusError struct {
	code  int
	cause error
}

func (e statusError) Error() string {
	if e.cause != nil {
		return e.cause.Error()
	}
	return fmt.Sprintf("exit status %d", e.code)
}

func (e statusError) Unwrap() error {
	return e.cause
}

func ExitStatus(code int) error {
	return statusError{code: code}
}

// BusyBox applets that raise xfunc_error_retval keep printing through
// bb_perror_msg, so a chosen status has to travel with its diagnostic rather
// than replace it. The shell prints the message and returns the code.
func ExitStatusMessage(code int, cause error) error {
	return statusError{code: code, cause: cause}
}

func StatusCode(err error) (int, bool) {
	var status statusError
	if !errors.As(err, &status) {
		return 0, false
	}
	return status.code, true
}

// StatusMessage reports the diagnostic a status error carries, if it carries
// one. A bare ExitStatus says nothing, which is how `false` stays quiet.
func StatusMessage(err error) (string, bool) {
	var status statusError
	if !errors.As(err, &status) || status.cause == nil {
		return "", false
	}
	return status.cause.Error(), true
}
