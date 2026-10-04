//go:build !windows

package runtime

import (
	"context"
	"errors"
	"os"
	"syscall"
	"time"
)

// lockHostFile is flock(2)'s lock, exclusive or shared, and reports whether it took it: with
// wait false, a lock someone else holds is no lock and no error. It asks without blocking and
// asks again, so Ctrl-C can end the wait.
func lockHostFile(ctx context.Context, file *os.File, shared, wait bool) (bool, error) {
	how := syscall.LOCK_EX
	if shared {
		how = syscall.LOCK_SH
	}
	for {
		err := syscall.Flock(int(file.Fd()), how|syscall.LOCK_NB)
		if err == nil {
			return true, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			return false, err
		}
		if !wait {
			return false, nil
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

// unlockHostFile lets the lock go.
func unlockHostFile(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}
