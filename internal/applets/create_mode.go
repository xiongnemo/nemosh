package applets

import "os"

// createMode is perm as a file or directory an applet makes asks for it. Windows has no umask,
// and a mode without the owner's write bit makes a read-only file there, so the shell's umask
// clears nothing from it; the view is where a shell's umask would come from.
func createMode(_ ProcessView, perm os.FileMode) os.FileMode {
	return perm
}

// createFile is os.Create through createMode.
func createFile(view ProcessView, native string) (*os.File, error) {
	return os.OpenFile(native, os.O_RDWR|os.O_CREATE|os.O_TRUNC, createMode(view, 0o666))
}
