//go:build windows

package applets

import "golang.org/x/sys/windows"

// defaultWindowsUID is busybox-w32's DEFAULT_UID (include/mingw.h:22). Keeping
// the same number means a script that compares against it behaves the same
// under either shell.
const defaultWindowsUID = 4095

// currentUserID maps "am I privileged" onto Windows elevation, which is
// busybox-w32's rule (win32/mingw.c:1292): zero only when the process is
// elevated *and* the Administrators group is enabled in its token. Either alone
// is not enough -- a token can carry the group while the process runs
// unelevated, and answering 0 there would tell a prompt it may write anywhere.
func currentUserID() int {
	token := windows.GetCurrentProcessToken()
	if !token.IsElevated() {
		return defaultWindowsUID
	}
	administrators, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return defaultWindowsUID
	}
	member, err := token.IsMember(administrators)
	if err != nil || !member {
		return defaultWindowsUID
	}
	return 0
}
