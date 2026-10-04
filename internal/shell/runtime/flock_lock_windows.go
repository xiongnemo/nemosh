package runtime

import (
	"context"
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// flock's lock on Windows is LockFileEx's, as busybox-w32's is, on one byte far past any data
// rather than on the file's bytes. A Windows lock is mandatory: busybox-w32 locks [0, size), so
// under `flock f cat f` the command cannot read f, "Device or resource busy"; and an empty file
// it locks no byte at all, so two flocks of one empty lock file both go ahead, measured. One
// byte at the top of the range keeps out the other flocks and nothing else.
var flockByte = windows.Overlapped{Offset: 0xFFFFFFFF, OffsetHigh: 0x7FFFFFFF}

// lockHostFile takes the lock, exclusive or shared, and reports whether it did: with wait
// false, a lock someone else holds is no lock and no error. It asks without waiting and asks
// again, so Ctrl-C can end the wait. A lock this handle already holds is let go first, as
// flock(2) converts one rather than stacking a second.
func lockHostFile(ctx context.Context, file *os.File, shared, wait bool) (bool, error) {
	handle := windows.Handle(file.Fd())
	_ = unlockHostFile(file)
	flags := uint32(windows.LOCKFILE_FAIL_IMMEDIATELY)
	if !shared {
		flags |= windows.LOCKFILE_EXCLUSIVE_LOCK
	}
	for {
		overlapped := flockByte
		err := windows.LockFileEx(handle, flags, 0, 1, 0, &overlapped)
		if err == nil {
			return true, nil
		}
		if !errors.Is(err, windows.ERROR_LOCK_VIOLATION) && !errors.Is(err, windows.ERROR_IO_PENDING) {
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

// unlockHostFile lets the lock go; one not held is no error, as flock(2)'s is not.
func unlockHostFile(file *os.File) error {
	overlapped := flockByte
	err := windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, &overlapped)
	if errors.Is(err, windows.ERROR_NOT_LOCKED) {
		return nil
	}
	return err
}
