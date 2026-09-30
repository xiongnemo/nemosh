package applets

import (
	"os"
	"runtime"
)

// createMode is perm as a file or directory an applet makes asks for it: without the bits the
// shell's umask clears. On Linux and macOS the process's mask is the shell's and clears them
// already; a subshell's, `(umask 077; touch f)`, may clear more, and it is the mode asked for
// that clears those, for the applet runs in the shell's process. Outside a shell the process's
// mask is all there is.
func createMode(view ProcessView, perm os.FileMode) os.FileMode {
	holder, ok := view.(fileModeMaskView)
	if !ok {
		return perm
	}
	return maskedMode(perm, uint32(holder.FileModeMask()))
}

// maskedMode is perm without umask's bits. Windows has no umask, and a mode without the
// owner's write bit makes a read-only file there, so on Windows it is perm as it is.
func maskedMode(perm os.FileMode, umask uint32) os.FileMode {
	if runtime.GOOS == "windows" {
		return perm
	}
	return perm &^ os.FileMode(umask&0o777)
}

// createFile is os.Create through createMode.
func createFile(view ProcessView, native string) (*os.File, error) {
	return os.OpenFile(native, os.O_RDWR|os.O_CREATE|os.O_TRUNC, createMode(view, 0o666))
}
