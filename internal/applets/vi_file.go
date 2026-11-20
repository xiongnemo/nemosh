package applets

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// viFile is the file a buffer was read from and is written to.
//
// Its lines are read as ed reads them (readEdLines): a file whose every line ends in CRLF is
// read without the CRs and written back with them, `ff=dos`; any other keeps a CR as a
// character, shown as ^M, so a mixed file goes back out as it came, which is vim's rule. A new
// file is written with LF: busybox-w32 writes CRLF, but ed and nano here write LF, Debian's
// vim has `fileformats=unix,dos`, and a `#!/bin/bash` written in vi has to run under WSL's
// bash and Git's. UTF-16 is decoded on the way in and written back as UTF-8, as by ed.
type viFile struct {
	// name is as it was given, which the messages use; "" is busybox's "No file".
	name     string
	native   string
	crlf     bool
	readOnly bool
}

// viLoad reads name into lines, busybox's file_insert at startup: a file that is not there is
// a new one, one empty line and no message, and one that cannot be read is an empty buffer
// and the reason, said as busybox says it.
func viLoad(view ProcessView, name string, readOnly bool) (viFile, [][]rune, string) {
	file := viFile{name: name, readOnly: readOnly}
	empty := [][]rune{{}}
	if name == "" {
		return file, empty, ""
	}
	native, err := resolveHostPath(view, name)
	if err != nil {
		return file, empty, fmt.Sprintf("'%s' %s", name, causeText(err))
	}
	file.native = native
	info, err := os.Stat(native)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return file, empty, ""
	case err != nil:
		return file, empty, fmt.Sprintf("'%s' %s", name, causeText(err))
	case !info.Mode().IsRegular():
		return file, empty, fmt.Sprintf("'%s' is not a regular file", name)
	}
	// A file that cannot be written is opened read-only, as busybox's update_file_info has it.
	file.readOnly = file.readOnly || info.Mode().Perm()&0o200 == 0
	handle, err := os.Open(native)
	if err != nil {
		return file, empty, fmt.Sprintf("'%s' %s", name, causeText(err))
	}
	defer handle.Close()
	text, _, crlf, err := readEdLines(handle)
	if err != nil {
		return file, empty, fmt.Sprintf("can't read '%s'", name)
	}
	file.crlf = crlf
	lines := make([][]rune, 0, max(len(text), 1))
	for _, line := range text {
		lines = append(lines, []rune(line))
	}
	if len(lines) == 0 {
		lines = empty
	}
	return file, lines, ""
}

// viWrite writes lines to native, each ended as the file's are, and answers the bytes written.
func viWrite(view ProcessView, native string, lines [][]rune, crlf bool) (int, error) {
	text := make([]string, len(lines))
	for index, line := range lines {
		text[index] = string(line)
	}
	// A new file is 0666 through the umask, as busybox's vi opens one; one that is there keeps
	// its own.
	perm := createMode(view, 0o666)
	if info, err := os.Stat(native); err == nil {
		perm = info.Mode().Perm()
	}
	return writeEdLines(native, text, false, crlf, perm)
}
