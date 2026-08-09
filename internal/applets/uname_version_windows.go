//go:build windows

package applets

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// osReleaseAndVersion reports what `uname -r` and `uname -v` answer.
//
// busybox-w32 fills these from GetVersionEx (win32/uname.c:21-23), which is
// deprecated and lies without a compatibility manifest -- it reports 6.2 for
// Windows 10 unless the binary declares support for something newer. busybox
// ships such a manifest; Nemosh does not, so the same call would report the
// wrong version here.
//
// RtlGetVersion is the one that answers truthfully regardless, which is what
// x/sys documents it for. The values are the same ones busybox reports on a
// manifested build: major.minor as the release, the build number as the
// version, measured together as 10.0 and 19045 on Windows 10 19045.
func osReleaseAndVersion() (string, string) {
	info := windows.RtlGetVersion()
	if info == nil {
		return unameUnknown, unameUnknown
	}
	return fmt.Sprintf("%d.%d", info.MajorVersion, info.MinorVersion),
		fmt.Sprintf("%d", info.BuildNumber)
}
