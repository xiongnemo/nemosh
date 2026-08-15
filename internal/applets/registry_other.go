//go:build !windows

package applets

// platformApplets adds nothing here. See DefaultRegistry for why `su` is absent
// rather than present-and-refusing: the name belongs to util-linux on this side,
// and shadowing it would take away a working command to offer a broken one.
func platformApplets() []Applet { return nil }
