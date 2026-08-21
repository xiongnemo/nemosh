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

// IsClosedPipe reports whether an error is a write to a pipe whose reader has gone away.
//
// Exported because the direct-dispatch path in cmd/nemosh has to reach the same conclusion, and it
// is a conclusion rather than a detail: `producer | head -1` is not a failure of the producer.
// POSIX kills the writer with SIGPIPE and it says nothing; Windows has no SIGPIPE, so the write
// simply fails and whoever gets that error has to recognise it for what it is.
//
// Measured, because this diverged: with a child shell producing and `grep -q` consuming,
// busybox-w32 printed nothing and this printed "seq: write /dev/stdout: The pipe is being closed."
// The classifier below already existed and this path had not been given it.
func IsClosedPipe(err error) bool { return isClosedPipeError(err) }
