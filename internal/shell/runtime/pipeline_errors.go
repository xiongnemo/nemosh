package runtime

import (
	"errors"
	"io/fs"
	"os"
	"syscall"
)

var errPipelineDownstreamClosed = errors.New("pipeline downstream closed")

func normalizePipelineWriteError(err error) error {
	if err == nil || !isClosedPipeError(err) {
		return err
	}
	return errPipelineDownstreamClosed
}

func isClosedPipeError(err error) bool {
	if errors.Is(err, os.ErrClosed) || errors.Is(err, syscall.EPIPE) {
		return true
	}
	if pathErr, ok := errors.AsType[*fs.PathError](err); ok {
		return isClosedPipeError(pathErr.Err)
	}
	errno, ok := errors.AsType[syscall.Errno](err)
	if !ok {
		return false
	}
	return errno == 109 || errno == 232
}
