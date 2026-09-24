//go:build !windows

package runtime

import "io/fs"

// fileAttributes has nothing to read off Windows, where a dot is what hides a file.
func fileAttributes(fs.DirEntry) (hidden, system bool) { return false, false }
