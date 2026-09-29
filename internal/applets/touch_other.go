//go:build !windows

package applets

import (
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// accessTime is when the file was last read. It asks again, for x/sys's Stat_t, whose field is
// Atim on Linux and on macOS both, where syscall's is not.
func accessTime(native string, info os.FileInfo) time.Time {
	var stat unix.Stat_t
	if unix.Stat(native, &stat) != nil {
		return info.ModTime()
	}
	seconds, nanoseconds := stat.Atim.Unix()
	return time.Unix(seconds, nanoseconds)
}

// setLinkTimes sets a symbolic link's own times, as utimensat with AT_SYMLINK_NOFOLLOW does. A
// zero time is the link's own, read first, since UTIME_OMIT is not spelled alike everywhere.
func setLinkTimes(native string, access, modified time.Time) error {
	var stat unix.Stat_t
	if err := unix.Lstat(native, &stat); err != nil {
		return &os.PathError{Op: "lstat", Path: native, Err: err}
	}
	stamps := []unix.Timespec{stat.Atim, stat.Mtim}
	for index, when := range []time.Time{access, modified} {
		if !when.IsZero() {
			stamps[index] = unix.NsecToTimespec(when.UnixNano())
		}
	}
	if err := unix.UtimesNanoAt(unix.AT_FDCWD, native, stamps, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return &os.PathError{Op: "chtimes", Path: native, Err: err}
	}
	return nil
}
