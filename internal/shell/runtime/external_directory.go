package runtime

import (
	"errors"
	"fmt"
)

// maxChildWorkingDirectory is the longest working directory a child process can
// be launched with. CreateProcess takes lpCurrentDirectory as an ordinary
// MAX_PATH buffer, so the directory has to leave room for a trailing separator
// and the NUL: 260 - 2. Measured on Windows 10 19045, 258 launches and 259 does
// not, and -- unlike every filesystem call Go makes on Nemosh's behalf -- the
// \\?\ prefix does not widen it. The budget is spent in UTF-16 code units,
// because that is what CreateProcess is handed; a 642-byte directory of 258 CJK
// characters launches, so counting Nemosh's UTF-8 bytes would reject paths
// Windows accepts.
//
// busybox-w32 has nothing to copy here: its cwd is SetCurrentDirectory, which is
// MAX_PATH-bound outright (win32/mingw.c:1703), so it cannot reach a directory
// this deep in the first place. Nemosh's cwd is virtual, so it can.
const maxChildWorkingDirectory = 258

var errNoShortWorkingDirectory = errors.New("no 8.3 short name is available")

// childWorkingDirectory answers what to hand CreateProcess as the child's
// working directory. A directory that fits goes through untouched; a longer one
// is retried as its 8.3 short name, which is the one form CreateProcess accepts
// for a directory past MAX_PATH. 8.3 generation can be switched off per volume,
// and then there is no way to launch from here at all -- so the diagnostic names
// the real constraint rather than letting Windows report "The directory name is
// invalid", which says nothing about length.
func childWorkingDirectory(dir string, shorten func(string) (string, error)) (string, error) {
	length := wideLength(dir)
	if length <= maxChildWorkingDirectory {
		return dir, nil
	}
	short, err := shorten(dir)
	if err == nil && wideLength(short) > maxChildWorkingDirectory {
		err = errNoShortWorkingDirectory
	}
	if err != nil {
		return "", fmt.Errorf("working directory is too long for a child process (%d > %d): %w",
			length, maxChildWorkingDirectory, err)
	}
	return short, nil
}

// wideLength counts the UTF-16 code units a string occupies once converted at
// the Windows API boundary. A byte outside UTF-8 becomes U+FFFD, one unit, which
// is what the conversion produces too.
func wideLength(s string) int {
	units := 0
	for _, r := range s {
		units++
		if r > 0xFFFF {
			units++
		}
	}
	return units
}
