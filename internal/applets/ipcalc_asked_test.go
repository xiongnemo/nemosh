package applets

import "testing"

// ipcalc asks what busybox's asks before it reads the address: with neither -b, -n nor -p
// there must be an -m or an -h, and no NETMASK. Either is busybox's usage, which -s does not
// silence; here it is one line, and status 1. `ipcalc -s 300.1.1.1` said nothing, and a
// NETMASK beside -m alone was printed back. busybox's long options are taken by any prefix
// that names one alone. Each was measured against busybox-w32.
func TestIpcalc_asksWhatBusyboxAsksBeforeTheAddress(t *testing.T) {
	t.Parallel()
	const nothing = "nothing was asked for; use -b, -n, -m, -p or -h"
	const netmask = "a NETMASK is only used by -b, -n or -p"
	for _, test := range []struct {
		args       []string
		out, error string
		status     int
	}{
		{args: []string{"-s", "300.1.1.1"}, error: nothing, status: 1},
		{args: []string{"300.1.1.1"}, error: nothing, status: 1},
		{args: []string{"-m", "10.0.0.5", "255.255.255.0"}, error: netmask, status: 1},
		{args: []string{"-h", "127.0.0.1", "255.0.0.0"}, error: netmask, status: 1},
		{args: []string{"-s", "1.2.3.4", "255.0.0.0"}, error: nothing, status: 1},
		{args: []string{"-bs", "300.1.1.1"}, status: 1},
		{args: []string{"-m", "300.1.1.1"}, error: "bad IP address: 300.1.1.1", status: 1},
		{args: []string{"-m", "10.0.0.5"}, out: "NETMASK=255.0.0.0\n"},
		{args: []string{"-mp", "10.0.0.5", "255.255.0.0"}, out: "NETMASK=255.255.0.0\nPREFIX=16\n"},
		{args: []string{"--broadcast", "1.1.1.1"}, out: "BROADCAST=1.255.255.255\n"},
		{args: []string{"--pre", "10.0.0.1/8"}, out: "PREFIX=8\n"},
		{args: []string{"--silent", "--network", "999.1.1.1"}, status: 1},
	} {
		out, stderr, status := runApplet(t, "ipcalc", test.args, "")
		if out != test.out || stderr != test.error || status != test.status {
			t.Errorf("ipcalc %q = %q, %q, status %d; want %q, %q, status %d", test.args, out, stderr, status, test.out, test.error, test.status)
		}
	}
}
