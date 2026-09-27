package runtime

import (
	"errors"
	"os"
	"path/filepath"
)

// `. name` without a slash looks for name on PATH first, as both references do, and a file
// found there need not be executable. It was read from the working directory instead, so
// `. lib.sh` with a lib.sh on PATH ran a lib.sh that happened to be beside the script, or
// failed. Not found on PATH, the name is read as it stands, which is bash's second step;
// busybox stops at PATH. `shopt -u sourcepath` leaves the search out, as it does in bash.

var errNotRegularFile = errors.New("not a regular file")

// dotSource is the native path of the file `. name` reads.
func (r Runtime) dotSource(name string) (string, error) {
	if r.options.sourcePath && !hasPathSeparator(name) {
		if found, ok := r.readableOnPath(name); ok {
			return found, nil
		}
	}
	resolved, err := r.ResolveNemoshPath(name)
	if err != nil {
		return "", err
	}
	if resolved.Device {
		return "", errNotRegularFile
	}
	return resolved.Native, nil
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
// answer; bash keeps what the file set. Given none, the file shares the caller's.
func (r Runtime) withDotArguments(arguments []string, run func() lineResult) lineResult {
	if len(arguments) == 0 {
		return run()
	}
	saved := r.params.values
	r.params.values = append([]string(nil), arguments...)
	defer func() { r.params.values = saved }()
	return run()
}
