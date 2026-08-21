package runtime

import (
	"io/fs"
)

// Describing a device to whatever asked, rather than refusing the path.
//
// Stage 1 of docs/design/device-filesystem.md. Before this, a device was openable and not
// observable: `cat /dev/null` worked while `test -e /dev/null` said no and `ls -l /dev/null`
// answered "is not a host path". Two divergences from busybox-w32 on paths this shell already
// opened correctly.
//
// The seam is the one opening already uses. `internal/applets` asks the process view rather than
// knowing which names are devices -- it must not gain a list of its own, or there would be two
// again, which is the whole reason Stage 0 collapsed four into one.

// StatProcessPath describes a path when it names a device, and says so when it does not.
//
// Three answers rather than two, and the middle one is the useful one: `false, nil` means "not a
// device, ask the filesystem", which is what lets the caller fall through to os.Stat without this
// method needing to know how to reach a disk.
func (r Runtime) StatProcessPath(path string) (fs.FileInfo, bool, error) {
	resolved, err := r.ResolveNemoshPath(path)
	if err != nil {
		return nil, false, err
	}
	if !resolved.Device {
		return nil, false, nil
	}
	name := string(resolved.Canonical)
	if info, ok := StatDevice(name); ok {
		return info, true, nil
	}
	// A descriptor alias: `/dev/stdin`, `/dev/stdout`, `/dev/stderr`, `/dev/fd/N`. They exist as
	// far as a test is concerned -- a script asking `test -e /dev/stdout` wants yes -- but they
	// are this process's own descriptors rather than devices with contents, which is why they
	// are not in the table. A malformed one (`/dev/fd/x`) reports the parse error rather than
	// claiming to exist.
	source, alias, aliasErr := deviceAlias(name)
	if aliasErr != nil {
		return nil, false, aliasErr
	}
	if alias {
		_ = source
		return deviceInfo{name: name}, true, nil
	}
	// Under /dev and not a device this shell has: `/dev/nosuchthing`. Not a device, and not a
	// host path either, so the caller's fall-through will refuse it -- which is the same answer
	// a name that does not exist gets anywhere else.
	return nil, false, nil
}
