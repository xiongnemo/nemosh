package runtime

import (
	"io"
	"os"
	"path/filepath"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// `. name` without a slash looks for name on PATH first, as both references do, and a file
// found there need not be executable. It was read from the working directory instead, so
// `. lib.sh` with a lib.sh on PATH ran a lib.sh that happened to be beside the script, or
// failed. Not found on PATH, the name is read as it stands, which is bash's second step;
// busybox stops at PATH. `shopt -u sourcepath` leaves the search out, as it does in bash.

// dotSource is what `. name` reads: the native path of a file, or the name of a device.
func (r Runtime) dotSource(name string) (string, string, error) {
	if r.options.sourcePath && !hasPathSeparator(name) {
		if found, ok := r.readableOnPath(name); ok {
			return found, "", nil
		}
	}
	resolved, err := r.ResolveNemoshPath(name)
	if err != nil {
		return "", "", err
	}
	if resolved.Device {
		return "", string(resolved.Canonical), nil
	}
	return resolved.Native, "", nil
}

// readDotSource is the text `. name` runs. A device is read to its end as a file is, as both
// references read it: `. /dev/null` is an empty script, and `. /dev/stdin <<EOF` and `... |
// . /dev/stdin` run what arrives on the shell's input. Each was "not a regular file".
func (r Runtime) readDotSource(native, device string) ([]byte, error) {
	// A directory is refused as it is opened, as every reader here refuses one; Windows lets Go
	// open it, and the read failed with `Incorrect function`.
	if device == "" {
		file, err := applets.OpenHostInput(native)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		return io.ReadAll(file)
	}
	fd, alias, err := deviceAlias(device)
	if err != nil {
		return nil, err
	}
	if alias {
		reader, err := r.fds.reader(fd)
		if err != nil {
			return nil, err
		}
		return io.ReadAll(reader)
	}
	resource, err := openInputDevice(device)
	if err != nil {
		return nil, err
	}
	defer resource.Close()
	return io.ReadAll(resource)
}

// readableOnPath is the first regular file called name in a directory on PATH.
func (r Runtime) readableOnPath(name string) (string, bool) {
	for _, directory := range filepath.SplitList(r.vars["PATH"]) {
		if directory == "" {
			directory = "."
		}
		resolved, err := r.ResolveNemoshPath(directory)
		if err != nil || resolved.Device {
			continue
		}
		candidate := filepath.Join(resolved.Native, name)
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
			return candidate, true
		}
	}
	return "", false
}

// withDotArguments runs a sourced file with its own positional parameters when `.` was given
// any, and gives the caller's back afterwards -- even ones the file set, which is busybox's
// answer; bash keeps what the file set. Given none, the file shares the caller's. Where
// getopts was comes back with them, as busybox restores it too.
func (r Runtime) withDotArguments(arguments []string, run func() lineResult) lineResult {
	if len(arguments) == 0 {
		return run()
	}
	saved, place := r.params.values, r.params.getopts
	r.params.values = append([]string(nil), arguments...)
	defer func() { r.params.values, r.params.getopts = saved, place }()
	return run()
}
