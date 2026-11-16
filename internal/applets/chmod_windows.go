package applets

import (
	"os"
	"syscall"
)

// chmodExecutable and chmodWrite are how chmod judges a file a program, which reads its first
// bytes, and sets what Windows keeps of a mode; a test counts both.
var (
	chmodExecutable = isExecutableFile
	chmodWrite      = applyPermissions
)

// currentPermissions is a file's mode as busybox-w32's stat makes one up (win32/mingw.c:390-400
// and 778-784), since Windows keeps no such bits: read for everyone, and write for everyone
// unless the file is read-only, less the umask's group and other write bits. A directory can
// always be written and run, and so can a file with a program's name. chmod's relative modes,
// -c and -v reckon from that, as busybox's do.
func currentPermissions(native string, info os.FileInfo, umask uint32) uint32 {
	return permissionsOf(native, info, umask, isExecutableFile)
}

// permissionsOf is currentPermissions with executable as the judge of a program.
func permissionsOf(native string, info os.FileInfo, umask uint32, executable func(string, os.FileInfo) bool) uint32 {
	writable := uint32(0o222) &^ (umask & 0o022)
	if info.IsDir() {
		return 0o555 | writable
	}
	mode := uint32(0o444)
	if info.Mode().Perm()&0o200 != 0 {
		mode |= writable
	}
	if executable(native, info) {
		mode |= 0o111
	}
	return mode
}

// chmodReckoned is the mode chmod reckons a file has. Its execute bits take a read of the file
// to judge, and they are wanted only to be said, by -v and -c: Windows keeps the owner's write bit
// and nothing else, and no MODE makes that bit hang on the execute bits -- not `X`, which sets
// only execute bits, nor `u=g`, which copies a write bit that is itself the read-only attribute.
// Without -v or -c a file is judged a program by its name alone, which reads nothing.
func chmodReckoned(native string, info os.FileInfo, umask uint32, said bool) uint32 {
	if said {
		return permissionsOf(native, info, umask, chmodExecutable)
	}
	return permissionsOf(native, info, umask, func(native string, info os.FileInfo) bool {
		return info.IsDir() || executableSuffix(native)
	})
}

// chmodApply sets what Windows keeps of mode, and only when the attributes info was read with
// say it is not so already: a chmod that changes nothing writes nothing.
func chmodApply(native string, info os.FileInfo, mode uint32) error {
	if data, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		readOnly := data.FileAttributes&syscall.FILE_ATTRIBUTE_READONLY != 0
		wantReadOnly := mode&0o200 == 0 && !info.IsDir()
		if readOnly == wantReadOnly {
			return nil
		}
	}
	return chmodWrite(native, mode, info.IsDir())
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
