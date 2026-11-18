//go:build windows

package applets

import (
	"os"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// defaultWindowsUID is busybox-w32's DEFAULT_UID (include/mingw.h:22). Keeping
// the same number means a script that compares against it behaves the same
// under either shell.
const defaultWindowsUID = 4095

// currentUserID maps "am I privileged" onto Windows elevation, which is
// busybox-w32's rule (win32/mingw.c:1292): zero only when the process is
// elevated *and* the Administrators group is enabled in its token. Either alone
// is not enough -- a token can carry the group while the process runs
// unelevated, and answering 0 there would tell a prompt it may write anywhere.
//
// Asked once: a process's elevation does not change. The question goes to ntdll, which every
// process has loaded, and not to advapi32's GetTokenInformation, whose loading -- advapi32,
// sechost, msvcrt and bcrypt -- was most of what $UID cost every start. Only an elevated
// process goes on to advapi32, for the membership.
func currentUserID() int { return processUserID() }

var processUserID = sync.OnceValue(func() int {
	if !processElevated() {
		return defaultWindowsUID
	}
	administrators, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return defaultWindowsUID
	}
	member, err := windows.GetCurrentProcessToken().IsMember(administrators)
	if err != nil || !member {
		return defaultWindowsUID
	}
	return 0
})

var procNtQueryInformationToken = windows.NewLazySystemDLL("ntdll.dll").NewProc("NtQueryInformationToken")

// processElevated is TokenElevation of this process's token, by NtQueryInformationToken on the
// pseudo-handle GetCurrentProcessToken names, which the kernel takes since Windows 8.
func processElevated() bool {
	var elevation uint32
	var length uint32
	status, _, _ := procNtQueryInformationToken.Call(uintptr(windows.GetCurrentProcessToken()), uintptr(windows.TokenElevation),
		uintptr(unsafe.Pointer(&elevation)), unsafe.Sizeof(elevation), uintptr(unsafe.Pointer(&length)))
	return status == 0 && elevation != 0
}

var procGetUserNameW = windows.NewLazySystemDLL("advapi32.dll").NewProc("GetUserNameW")

// processAccountName is what Windows calls the logged-in user: %USERNAME%, which it sets for every
// logon, or else GetUserNameW, as busybox-w32's get_user_name asks. os/user.Current answered the
// same name, and loaded userenv, profapi, secur32 and sspicli to work out a profile directory
// and a display name that were thrown away.
var processAccountName = sync.OnceValue(func() string {
	if name := os.Getenv("USERNAME"); name != "" {
		return name
	}
	var buffer [257]uint16
	size := uint32(len(buffer))
	if result, _, _ := procGetUserNameW.Call(uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size))); result == 0 {
		return ""
	}
	return windows.UTF16ToString(buffer[:])
})
