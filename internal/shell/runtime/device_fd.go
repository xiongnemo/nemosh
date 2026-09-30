package runtime

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"syscall"
)

var errMalformedDeviceFD = errors.New("malformed /dev/fd descriptor")

// noSuchDevice is a name under /dev that is none of this shell's devices. It is not there, as
// busybox-w32 answers, so it is fs.ErrNotExist to whatever asks: `cat: cannot open
// '/dev/nosuch': No such file or directory`, and for a redirection `nonexistent directory`. It
// was "unsupported device", with the name twice: `/dev/nosuch: /dev/nosuch: unsupported device`.
type noSuchDevice struct{ path string }

func (e noSuchDevice) Error() string        { return e.path + ": No such file or directory" }
func (e noSuchDevice) Is(target error) bool { return target == fs.ErrNotExist }

// missingDevice is what opening path answers when no device of this shell's is called that:
// /dev itself is a directory, and any other name under it is not there.
func missingDevice(path string) error {
	if path == "/dev" {
		return &fs.PathError{Op: "open", Path: path, Err: syscall.EISDIR}
	}
	return noSuchDevice{path: path}
}

func deviceAlias(path string) (int, bool, error) {
	switch path {
	case "/dev/stdin":
		return 0, true, nil
	case "/dev/stdout":
		return 1, true, nil
	case "/dev/stderr":
		return 2, true, nil
	}
	if !strings.HasPrefix(path, "/dev/fd/") {
		return 0, false, nil
	}
	text := strings.TrimPrefix(path, "/dev/fd/")
	if text == "" || !isDigits(text) {
		return 0, false, fmt.Errorf("%s: %w", path, errMalformedDeviceFD)
	}
	fd, err := parseDescriptor(text, -1)
	if err != nil {
		return 0, false, fmt.Errorf("%s: %w", path, err)
	}
	return fd, true, nil
}

// isVirtualDevice reports whether the path names one of the devices with contents.
//
// From the table now, rather than a fourth copy of the same list of names. A name in the opener and
// missing here was openable and unrecognised; missing there and present here, recognised and
// unopenable. Neither can happen with one list.
func isVirtualDevice(path string) bool {
	_, found := lookupDevice(path)
	return found
}
