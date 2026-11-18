package applets

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// pathIsRemote is whether a host path is on another machine, which a test makes up.
var pathIsRemote = isRemotePath

// driveTypeOf is GetDriveTypeW of a root such as `Z:\`, which a test makes up.
var driveTypeOf = func(root string) uint32 {
	name, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return windows.DRIVE_UNKNOWN
	}
	return windows.GetDriveType(name)
}

// isRemotePath is whether a host path is on another machine: a UNC path, `\\server\share\...` or
// `\\?\UNC\server\share\...`, or one on a drive Windows maps to a share, which GetDriveTypeW calls
// DRIVE_REMOTE. A directory on a local drive that a link points at a share is not seen through;
// finding that would cost an open of the path.
func isRemotePath(host string) bool {
	volume := filepath.VolumeName(host)
	switch {
	case hasPrefixFold(volume, `\\?\UNC\`):
		return true
	case strings.HasPrefix(volume, `\\?\`):
		volume = volume[len(`\\?\`):]
	case strings.HasPrefix(volume, `\\.\`):
		// The device namespace: pipes, NUL, a volume by its device name. None is a share.
		return false
	case strings.HasPrefix(volume, `\\`):
		return true
	}
	if len(volume) != 2 || volume[1] != ':' {
		return false
	}
	return driveTypeOf(volume+`\`) == windows.DRIVE_REMOTE
}

func hasPrefixFold(text, prefix string) bool {
	return len(text) >= len(prefix) && strings.EqualFold(text[:len(prefix)], prefix)
}
