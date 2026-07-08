package applets

import (
	"errors"
	"fmt"
)

var ErrExitFalse = errors.New("false")

type statusError struct {
	code int
}

func (e statusError) Error() string {
	return fmt.Sprintf("exit status %d", e.code)
}

func ExitStatus(code int) error {
	return statusError{code: code}
}

func StatusCode(err error) (int, bool) {
	var status statusError
	if !errors.As(err, &status) {
		return 0, false
	}
	return status.code, true
}
