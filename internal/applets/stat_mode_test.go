package applets

import "testing"

// %A is libbb's bb_mode_string: a set-id or sticky bit takes the execute place, in capitals
// where that is not set.
func TestStatModeString_isBusyboxs(t *testing.T) {
	for mode, want := range map[uint32]string{
		statRegular | 0o644:    "-rw-r--r--",
		statRegular | 0o4755:   "-rwsr-xr-x",
		statRegular | 0o2644:   "-rw-r-Sr--",
		statDirectory | 0o1777: "drwxrwxrwt",
		statDirectory | 0o1776: "drwxrwxrwT",
		statCharacter | 0o666:  "crw-rw-rw-",
		statLink | 0o777:       "lrwxrwxrwx",
		statFIFO | 0o600:       "prw-------",
		statSocket | 0o755:     "srwxr-xr-x",
		statBlock | 0o660:      "brw-rw----",
		0o644:                  "?rw-r--r--",
	} {
		if got := statModeString(mode); got != want {
			t.Errorf("statModeString(%o) = %q, want %q", mode, got, want)
		}
	}
}
