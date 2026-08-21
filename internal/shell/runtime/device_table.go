package runtime

import (
	"crypto/rand"
	"io"
	"io/fs"
	"path"
	"time"
)

// The devices, as one table.
//
// They were four `switch` statements over the same names -- one for reading through a redirect, one
// for reading as an operand, one for writing, and one asking only whether a name was a device at
// all. Four lists of the same nine strings, which is three chances for them to disagree: a device
// added to the opener and forgotten in `isVirtualDevice` would be openable and unrecognised, and one
// forgotten the other way round would be recognised and unopenable.
//
// This is the first step of `docs/design/device-filesystem.md`, and on its own it changes no
// behaviour whatever. What it buys is the thing the rest of that document needs: a single list to
// answer from. `test -e /dev/zero` cannot disagree with `cat /dev/zero` once both read this.
//
// `/dev/stdin`, `/dev/stdout`, `/dev/stderr` and `/dev/fd/N` are deliberately *not* here. They are
// not devices with contents; they are other names for descriptors this process already holds, and
// `deviceAlias` in device_fd.go resolves them to a number before anything reaches this table.

// virtualDevice is one device: what it is called, and how it may be opened.
//
// A nil opener means the device does not work that way, and the error the caller then produces is
// the same one an unknown name gets. That is intentional: `> /dev/zero` and `> /dev/nosuchthing`
// are both "you cannot write there", and inventing a second wording for the first would suggest a
// distinction the shell does not make.
type virtualDevice struct {
	name      string
	openRead  func() (io.ReadCloser, error)
	openWrite func(appendMode bool) (io.WriteCloser, error)
}

// virtualDevices is every device this shell provides, in the order `/dev` will list them once
// listing exists. Alphabetical, because that is what a directory listing looks like.
var virtualDevices = []virtualDevice{
	{
		name: "/dev/clipboard",
		// The only device where appending differs from truncating: the others hold
		// nothing to append to, so `>>` and `>` mean the same thing there.
		openRead:  openClipboardReader,
		openWrite: openClipboardWriter,
	},
	{
		name:      "/dev/null",
		openRead:  func() (io.ReadCloser, error) { return nullDevice{}, nil },
		openWrite: func(bool) (io.WriteCloser, error) { return nullDevice{}, nil },
	},
	{
		name:     "/dev/random",
		openRead: openRandomReader,
	},
	{
		name:     "/dev/urandom",
		openRead: openRandomReader,
	},
	{
		name:     "/dev/zero",
		openRead: func() (io.ReadCloser, error) { return io.NopCloser(zeroReader{}), nil },
	},
}

// openRandomReader is crypto/rand for both spellings.
//
// `/dev/random` and `/dev/urandom` differ on Linux -- one can block waiting for entropy -- and on
// Windows there is one source and it does not block, so the distinction has nothing to represent
// here. Both names work because scripts use both.
func openRandomReader() (io.ReadCloser, error) { return io.NopCloser(rand.Reader), nil }

// lookupDevice finds a device by its full path.
func lookupDevice(path string) (virtualDevice, bool) {
	for _, device := range virtualDevices {
		if device.name == path {
			return device, true
		}
	}
	return virtualDevice{}, false
}

// deviceInfo is what a device looks like to stat.
//
// Synthetic, because there is nothing on disk to ask -- and checked against the platform rather
// than invented: os.Stat("NUL") on Windows reports `Dcrw-rw-rw-` with size zero, and busybox-w32
// prints `crw-rw-rw-` for /dev/null under its own shell. A character device with mode 0666 and no
// contents is what both of them say, so it is what this says.
//
// The modification time is the epoch, which is also busybox's answer (`Jan 01 1970`). A device has
// no meaningful one, and today's date would imply it had just changed.
type deviceInfo struct {
	name string
}

func (i deviceInfo) Name() string { return path.Base(i.name) }
func (i deviceInfo) Size() int64  { return 0 }

// Mode is what makes `test -c` true and `test -f` false, which is the distinction a script cares
// about: a device is not a regular file and reading it does not end.
func (i deviceInfo) Mode() fs.FileMode  { return fs.ModeDevice | fs.ModeCharDevice | 0o666 }
func (i deviceInfo) ModTime() time.Time { return time.Unix(0, 0).UTC() }
func (i deviceInfo) IsDir() bool        { return false }
func (i deviceInfo) Sys() any           { return nil }

// StatDevice describes a device path, and reports whether it is one at all.
//
// Exported because the applets reach it through the process view, the same way they already reach
// opening: internal/applets holds no knowledge of which names are devices and must not gain any,
// or there would be two lists again.
func StatDevice(path string) (fs.FileInfo, bool) {
	if _, found := lookupDevice(path); found {
		return deviceInfo{name: path}, true
	}
	return nil, false
}
