package runtime

import "strings"

// The four names that mean "a descriptor this shell holds".
//
// Kept apart from the device table because they are a different kind of thing, and the difference
// decides which platforms get them. A device is hardware, or an invention standing in for hardware,
// and where the system provides its own this shell must not shadow it. A descriptor alias is not
// hardware at all: `/dev/stdout` means *this shell's* standard output, which after a redirect is not
// descriptor 1 of the process, and which in this shell's fd table may not be an operating-system
// file at all -- a pipe it made, an in-memory buffer, the clipboard.
//
// So every platform gets these, and only Windows gets the devices. bash documents the same two
// routes for itself: it uses the platform's special files where they exist and emulates them where
// they do not. Emulating is what keeps the fd table authoritative, which is the whole reason the
// shell has one.
func isDescriptorAliasPath(path string) bool {
	switch path {
	case "/dev/stdin", "/dev/stdout", "/dev/stderr":
		return true
	}
	return strings.HasPrefix(path, "/dev/fd/")
}
