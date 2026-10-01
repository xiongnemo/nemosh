package applets

import (
	"archive/tar"
	"fmt"
	"io"
	"strconv"
)

// What -v says, as busybox says it. verbose counts -t and -v together, as busybox's
// verboseFlag does: one names each entry, two give the long line, so `tar tv` and `tar xvv`
// are long and `tar xv` names. Creating names each entry however many there are.
//
// The names go to stdout, as busybox prints them, but to stderr where stdout carries the
// archive, which busybox does for creating, or the data, under -O, where busybox mixes the
// names into the data. They all went to stderr, and the long line was Go's mode, the size
// and the name.

// tarListing is busybox's long line: the mode as ls has it, the owner and group by name or
// else by number, the size, the local time, the name, and where a link of either kind points.
func tarListing(header *tar.Header) string {
	user, group := header.Uname, header.Gname
	if user == "" {
		user = strconv.Itoa(header.Uid)
	}
	if group == "" {
		group = strconv.Itoa(header.Gid)
	}
	line := fmt.Sprintf("%s %s/%s %9d %s %s", statModeString(tarMode(header)), user, group, header.Size,
		header.ModTime.Local().Format("2006-01-02 15:04:05"), header.Name)
	if header.Linkname != "" {
		line += " -> " + header.Linkname
	}
	return line
}

// tarMode is an entry's mode with its type in it, which the archive keeps apart: a hard link
// is a file, as busybox lists it.
func tarMode(header *tar.Header) uint32 {
	kind := uint32(0o100000)
	switch header.Typeflag {
	case tar.TypeDir:
		kind = 0o040000
	case tar.TypeSymlink:
		kind = 0o120000
	case tar.TypeChar:
		kind = 0o020000
	case tar.TypeBlock:
		kind = 0o060000
	case tar.TypeFifo:
		kind = 0o010000
	}
	return kind | uint32(header.Mode)&0o7777
}

// sayExtracted is -v's line for an entry extracted.
func (r tarRequest) sayExtracted(stdout, stderr io.Writer, line string) {
	if r.verbose == 0 {
		return
	}
	if r.toStdout {
		stdout = stderr
	}
	fmt.Fprintln(stdout, line)
}
