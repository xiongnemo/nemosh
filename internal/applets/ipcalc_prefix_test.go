package applets

import "testing"

// ipcalc reads the prefix after a slash as busybox's xatoul_range reads it: a netmask there
// is no number, and 33 is a number out of the range. The netmask was said to be out of the
// range too. Each was measured against busybox-w32.
func TestIpcalc_readsThePrefixAsBusyboxsXatoulRange(t *testing.T) {
	t.Parallel()
	for prefix, want := range map[string]string{
		"255.255.255.0": "invalid number '255.255.255.0'",
		"x":             "invalid number 'x'",
		"33":            "number 33 is not in 0..32 range",
	} {
		if _, stderr, status := runApplet(t, "ipcalc", []string{"-n", "192.168.1.5/" + prefix}, ""); stderr != want || status != 1 {
			t.Errorf("ipcalc -n 192.168.1.5/%s = %q, status %d; want %q, status 1", prefix, stderr, status, want)
		}
	}
}
