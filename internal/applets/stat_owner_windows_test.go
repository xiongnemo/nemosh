package applets

import "testing"

// A local or domain account's relative ID is the last part of its SID's string, and any other
// SID has none. Read from the string, since x/sys/windows' SID accessors trip the race detector's
// pointer checks on a file another account owns, as CI's did.
func TestAccountRID_readsALocalOrDomainAccount(t *testing.T) {
	for _, test := range []struct {
		sid  string
		want uint32
		ok   bool
	}{
		{"S-1-5-21-3623811015-3361044348-30300820-1013", 1013, true},
		{"S-1-5-21-1-2-3-500", 500, true},
		{"S-1-5-32-544", 0, false},
		{"S-1-5-18", 0, false},
		{"S-1-5-21-1-2-500", 0, false},
		{"S-1-5-21-1-2-3-x", 0, false},
	} {
		if got, ok := accountRID(test.sid); got != test.want || ok != test.ok {
			t.Errorf("accountRID(%q) = %d, %v; want %d, %v", test.sid, got, ok, test.want, test.ok)
		}
	}
}
