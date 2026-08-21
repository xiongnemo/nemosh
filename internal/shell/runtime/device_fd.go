package runtime

import (
	"errors"
	"fmt"
	"strings"
)

var (
	errMalformedDeviceFD = errors.New("malformed /dev/fd descriptor")
	errUnsupportedDevice = errors.New("unsupported device")
)

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
