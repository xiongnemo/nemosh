//go:build windows

package applets

// platformApplets are the applets that only mean something on Windows.
func platformApplets() []Applet {
	return []Applet{newSuApplet()}
}
