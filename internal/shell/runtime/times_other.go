//go:build !windows

package runtime

import (
	"syscall"
	"time"
)

func processCPUTime() (time.Duration, time.Duration, error) {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return 0, 0, err
	}
	return timevalDuration(usage.Utime), timevalDuration(usage.Stime), nil
}

func timevalDuration(t syscall.Timeval) time.Duration {
	return time.Duration(t.Sec)*time.Second + time.Duration(t.Usec)*time.Microsecond
}
