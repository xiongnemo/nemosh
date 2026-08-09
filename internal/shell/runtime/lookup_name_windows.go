//go:build windows

package runtime

import (
	"errors"

	"golang.org/x/sys/windows"
)

// isUnusableName reports whether the error means the path could never name a
// file, as opposed to naming one that is missing.
//
// Windows distinguishes the two and Go passes the distinction through, but the
// shell was treating only ERROR_FILE_NOT_FOUND as "not there" and letting the
// rest surface. A command name carrying a carriage return, a control character,
// or one of the characters Windows reserves then produced the raw CreateFile
// failure -- "The filename, directory name, or volume label syntax is
// incorrect" -- where busybox simply says `not found`.
//
// A name that cannot exist does not exist. Reporting it as anything else sends
// the reader looking for a filesystem problem that is not there.
func isUnusableName(err error) bool {
	var errno windows.Errno
	if !errors.As(err, &errno) {
		return false
	}
	switch errno {
	case windows.ERROR_INVALID_NAME, windows.ERROR_BAD_PATHNAME, windows.ERROR_FILENAME_EXCED_RANGE, windows.ERROR_DIRECTORY:
		return true
	}
	return false
}
