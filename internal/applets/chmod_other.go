//go:build !windows

package applets

import "os"

// currentPermissions is the file's own mode, its special bits included.
func currentPermissions(_ string, info os.FileInfo, _ uint32) uint32 {
	return bitsOfFileMode(info.Mode())
}

// applyPermissions sets the mode, special bits and all.
func applyPermissions(native string, mode uint32, _ bool) error {
	return os.Chmod(native, fileModeOfBits(mode))
}
