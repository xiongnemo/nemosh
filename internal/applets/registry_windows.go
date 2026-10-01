//go:build windows

package applets

// platformApplets are the applets that only mean something on Windows. lsattr and chattr are
// busybox-w32's, for Windows' file attributes; elsewhere the names are e2fsprogs', for flags
// these know nothing of.
func platformApplets() []Applet {
	return []Applet{newSuApplet(), newLsattrApplet(), newChattrApplet()}
}
