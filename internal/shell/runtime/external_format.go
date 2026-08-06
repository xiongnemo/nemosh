package runtime

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// hasWindowsExecutableSuffix mirrors busybox-w32 has_exe_suffix: the last dot in
// the base name, at most three characters of suffix, compared case-insensitively
// against the same five names the lookup appends.
func hasWindowsExecutableSuffix(name string) bool {
	return hasWindowsSuffix(name, windowsExecutableSuffixes[:])
}

// hasWindowsExecutableSuffixOrDot mirrors has_exe_suffix_or_dot. A trailing
// literal dot also blocks the suffix search, because Windows drops it when
// opening the file and the appended name would name a different file entirely.
func hasWindowsExecutableSuffixOrDot(name string) bool {
	return strings.HasSuffix(name, ".") || hasWindowsExecutableSuffix(name)
}

// hasExecutableFormat sniffs a file that carries no executable suffix, the way
// busybox does before deciding a bare name is runnable (win32/mingw.c:487).
// busybox goes on to walk the PE header; nemosh stops at the magic. The
// difference only shows up for a file that starts with MZ but is not a program,
// where the narrower check would trade one launch failure for another.
func hasExecutableFormat(path string) (bool, error) {
	// A DLL starts with MZ and is never runnable; busybox excludes it by name
	// rather than paying for the read on the thousands of them a system carries.
	if strings.EqualFold(filepath.Ext(path), ".dll") {
		return false, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("read executable %q: %w", path, err)
	}
	defer file.Close()
	header := make([]byte, 4)
	count, err := io.ReadFull(file, header)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return false, fmt.Errorf("read executable %q: %w", path, err)
	}
	// busybox wants at least four bytes before it will judge the format at all.
	if count < 4 {
		return false, nil
	}
	return bytes.HasPrefix(header, []byte("#!")) || bytes.HasPrefix(header, []byte("MZ")), nil
}
