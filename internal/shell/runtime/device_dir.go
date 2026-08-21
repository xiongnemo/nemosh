package runtime

import (
	"io/fs"
	"sort"
	"strings"
	"time"
)

// `/dev` as a directory.
//
// Stage 2 of docs/design/device-filesystem.md, and the one place this deliberately parts company
// with busybox-w32: `busybox ls /dev` answers "No such file or directory", because busybox provides
// the entries and not the directory. Listing was chosen anyway, and the reason is discoverability.
// Without it the only way to learn which devices exist is to read a document, and a shell whose own
// features are documented rather than visible has hidden them. The divergence is recorded in
// docs/support-matrix.md rather than left to be discovered.
//
// The descriptor aliases are listed even though they are not in the device table. Somebody reading
// `/dev` to find out what they can redirect to wants `stdout` in that list, and this shell does
// answer for the name -- it just answers by handing back a descriptor rather than opening a device.
// Leaving them out would make the listing a description of the implementation rather than of what
// works.

// devicePath is the directory itself.
const devicePath = "/dev"

// descriptorAliases are the names `/dev` lists in addition to the table's devices.
//
// `/dev/fd` is a directory of its own on a real system, holding one entry per open descriptor. It is
// listed as a name here and not enumerated: the contents change with every redirect, and a listing
// that depends on how it was invoked is a listing nobody can rely on.
var descriptorAliases = []string{"stderr", "stdin", "stdout"}

// directoryInfo is what `/dev` looks like to stat.
//
// Read-only and executable, which is what a directory needs to be listed and entered: 0555 rather
// than 0755, because nothing can be created in it. `touch /dev/foo` has nowhere to go, and a mode
// that implied otherwise would be a promise.
type directoryInfo struct{ name string }

func (i directoryInfo) Name() string      { return i.name }
func (i directoryInfo) Size() int64       { return 0 }
func (i directoryInfo) Mode() fs.FileMode { return fs.ModeDir | 0o555 }

// The epoch, as the devices report, and for the same reason: a synthetic directory has no
// modification time, and today's date would say it had just changed.
func (i directoryInfo) ModTime() time.Time { return time.Unix(0, 0).UTC() }
func (i directoryInfo) IsDir() bool        { return true }
func (i directoryInfo) Sys() any           { return nil }

// deviceEntry is one line of the listing.
type deviceEntry struct {
	name string
	info fs.FileInfo
}

func (e deviceEntry) Name() string               { return e.name }
func (e deviceEntry) IsDir() bool                { return e.info.IsDir() }
func (e deviceEntry) Type() fs.FileMode          { return e.info.Mode().Type() }
func (e deviceEntry) Info() (fs.FileInfo, error) { return e.info, nil }

// ReadDeviceDir lists `/dev`, and reports whether the path is that directory at all.
func ReadDeviceDir(path string) ([]fs.DirEntry, bool) {
	if strings.TrimSuffix(path, "/") != devicePath {
		return nil, false
	}
	entries := make([]fs.DirEntry, 0, len(virtualDevices)+len(descriptorAliases))
	for _, device := range virtualDevices {
		name := strings.TrimPrefix(device.name, devicePath+"/")
		entries = append(entries, deviceEntry{name: name, info: deviceInfo{name: device.name}})
	}
	for _, alias := range descriptorAliases {
		entries = append(entries, deviceEntry{
			name: alias,
			info: deviceInfo{name: devicePath + "/" + alias},
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return entries, true
}

// StatDeviceDir describes `/dev` itself.
func StatDeviceDir(path string) (fs.FileInfo, bool) {
	if strings.TrimSuffix(path, "/") != devicePath {
		return nil, false
	}
	return directoryInfo{name: "dev"}, true
}
