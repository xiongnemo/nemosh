package applets

import "os"

// currentPermissions is a file's mode as busybox-w32's stat makes one up (win32/mingw.c:390-400
// and 778-784), since Windows keeps no such bits: read for everyone, and write for everyone
// unless the file is read-only, less the umask's group and other write bits. A directory can
// always be written and run, and so can a file with a program's name. chmod's relative modes,
// -c and -v reckon from that, as busybox's do.
func currentPermissions(native string, info os.FileInfo, umask uint32) uint32 {
	writable := uint32(0o222) &^ (umask & 0o022)
	if info.IsDir() {
		return 0o555 | writable
	}
	mode := uint32(0o444)
	if info.Mode().Perm()&0o200 != 0 {
		mode |= writable
	}
	if isExecutableFile(native, info) {
		mode |= 0o111
	}
	return mode
}

// applyPermissions sets a mode as far as Windows holds one: the owner's write bit clears or
// sets the read-only attribute, and the rest is not kept. A directory is never made read-only,
// as busybox-w32's mingw_chmod has it (win32/mingw.c:1896), since on a folder the attribute
// means something else to Explorer.
func applyPermissions(native string, mode uint32, isDir bool) error {
	if isDir {
		mode |= 0o222
	}
	return os.Chmod(native, fileModeOfBits(mode))
}
