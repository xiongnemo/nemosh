package applets

import "os"

// IsExecutable answers what `test -x` answers about a file: the execute bit, which on Windows
// is busybox-w32's synthesised one (see isExecutableFile). Exported for completion, which
// offers a program typed by its path only when it could run.
func IsExecutable(native string, info os.FileInfo) bool {
	return isExecutableFile(native, info)
}
