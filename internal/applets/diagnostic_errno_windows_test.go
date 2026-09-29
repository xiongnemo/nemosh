package applets

import (
	"io/fs"
	"syscall"
	"testing"
)

// A Windows error is worded as busybox-w32 words it, by the errno its err_win_to_posix makes of
// it, and asking about a file as its get_file_attr answers. Windows' own sentences came through
// instead: a file in use was `The process cannot access the file because it is being used by
// another process.`, where busybox says `Permission denied`.
func TestCauseText_wordsAWindowsErrorAsBusyboxW32Does(t *testing.T) {
	for _, test := range []struct {
		op   string
		code syscall.Errno
		want string
	}{
		{"remove", 32, "Permission denied"}, // ERROR_SHARING_VIOLATION
		{"open", 33, "Permission denied"},   // ERROR_LOCK_VIOLATION
		{"open", 123, "Invalid argument"},   // ERROR_INVALID_NAME
		{"GetFileAttributesEx", 123, "No such file or directory"},
		{"CreateFile", 32, "Permission denied"},
		{"write", 112, "No space left on device"}, // ERROR_DISK_FULL
		{"write", 19, "Read-only file system"},    // ERROR_WRITE_PROTECT
		{"write", 109, "Broken pipe"},             // ERROR_BROKEN_PIPE
		{"remove", 145, "Directory not empty"},    // ERROR_DIR_NOT_EMPTY
		{"open", 206, "Filename too long"},        // ERROR_FILENAME_EXCED_RANGE
	} {
		err := &fs.PathError{Op: test.op, Path: "x", Err: test.code}
		if got := causeText(err); got != test.want {
			t.Errorf("%s failing with %d: got %q, want %q", test.op, uint32(test.code), got, test.want)
		}
	}
}
