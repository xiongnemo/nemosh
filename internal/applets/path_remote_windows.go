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
//
// The prefixes are read from the path itself, not from filepath.VolumeName, whose answer for
// `\\?\UNC\server\share\file` is not the same in Go 1.26 as in 1.27: CI's 1.26 made it a local
// path, and this machine's 1.27 did not.
func isRemotePath(host string) bool {
	host = filepath.FromSlash(host)
	switch {
	case hasPrefixFold(host, `\\?\UNC\`):
		return true
	case strings.HasPrefix(host, `\\.\`):
		// The device namespace: pipes, NUL, a volume by its device name. None is a share.
		return false
	case strings.HasPrefix(host, `\\?\`):
		host = host[len(`\\?\`):]
	case strings.HasPrefix(host, `\\`):
		return true
	}
	if len(host) < 2 || host[1] != ':' {
		return false
	}
	return driveTypeOf(host[:2]+`\`) == windows.DRIVE_REMOTE
}

func hasPrefixFold(text, prefix string) bool {
	return len(text) >= len(prefix) && strings.EqualFold(text[:len(prefix)], prefix)
}
