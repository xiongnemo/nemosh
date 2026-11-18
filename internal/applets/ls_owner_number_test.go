package applets

import (
	"testing"

	"golang.org/x/sys/windows"
)

// An account this machine cannot name -- a NAS's own user, seen over SMB as S-1-5-21-<the NAS's
// domain>-1000 -- is the number busybox-w32 gives it, its relative ID: `ls -l` there printed
// `root`, where busybox prints `1000` and this build's own `stat` and `ls -n` said uid 1000. An
// identity that is no account is root, busybox's uid 0, and an account that can be named is named.
func TestOwnerAccountName_anAccountWithNoNameIsItsNumber(t *testing.T) {
	self := processUserSID()
	if self == nil {
		t.Skip("no process user to compare with")
	}
	selfName, _, _, err := self.LookupAccount("")
	if err != nil {
		t.Skipf("this account cannot be looked up here: %v", err)
	}
	for sid, want := range map[string]string{
		"S-1-5-21-1-2-3-1000": "1000",
		"S-1-5-21-1-2-3-4094": "4094",
		"S-1-5-21-1-2-3-100":  "root",
		"S-1-5-21-1-2-3-4095": "root",
		"S-1-5-18":            "root",
		"S-1-22-1-1000":       "root",
		self.String():         selfName,
	} {
		parsed, err := windows.StringToSid(sid)
		if err != nil {
			t.Fatal(err)
		}
		if got := ownerAccountName(parsed); got != want {
			t.Errorf("the owner %s is named %q, want %q", sid, got, want)
		}
	}
}
