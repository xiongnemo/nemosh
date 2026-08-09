package applets

import "testing"

// The name has to follow the number.
//
// Under gsudo, `id -u` answered 0 -- the elevation check was right -- while every
// name still came from the Windows account, so the shell said `uid=0(nemo)` and
// a prompt using `\u` read `nemo` while its `\$` read `%`. One process, two
// answers. busybox does not have that problem because the name is derived from
// the uid rather than looked up beside it: getpwuid(0) is "root" and
// getpwuid(DEFAULT_UID) is the account name (win32/mingw.c:1313-1320).
//
// The mapping is tested rather than the elevation, because elevation is the one
// part of this a test cannot arrange for itself.
func TestIdentityName_followsTheUid(t *testing.T) {
	for _, test := range []struct {
		name    string
		uid     int
		account string
		want    string
	}{
		{name: "elevated is root", uid: 0, account: "nemo", want: "root"},
		{name: "elevated with no account name is still root", uid: 0, account: "", want: "root"},
		{name: "ordinary is the account", uid: 4095, account: "nemo", want: "nemo"},
		{name: "ordinary with no account name says nothing", uid: 4095, account: "", want: ""},
		// 4095 spelled out rather than named: the constant is Windows-only and
		// this rule is not. What the rule turns on is zero, not the default.
		{name: "any other uid is the account", uid: 1000, account: "nemo", want: "nemo"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := identityName(test.uid, test.account); got != test.want {
				t.Fatalf("identityName(%d, %q) = %q, want %q", test.uid, test.account, got, test.want)
			}
		})
	}
}

// Whatever the platform says, `id` and the prompt must say the same thing: the
// bug was them disagreeing, not either of them being wrong on its own.
func TestCurrentIdentity_agreesWithCurrentUserName(t *testing.T) {
	// When
	identity := currentIdentity()
	exported := CurrentUserName()

	// Then
	if exported != "" && identity.user != exported {
		t.Fatalf("id reports %q and the prompt would show %q", identity.user, exported)
	}
	if identity.user == "" {
		t.Fatal("id reported an empty user name, which is not an answer")
	}
	// The group follows the same rule, so `id -gn` cannot drift from `id -un`.
	if identity.group != identity.user {
		t.Fatalf("group %q and user %q disagree", identity.group, identity.user)
	}
	// And an elevated process must call itself root in both places at once.
	if identity.uid == 0 && identity.user != rootUserName {
		t.Fatalf("uid is 0 but the name is %q", identity.user)
	}
	if identity.uid != 0 && identity.user == rootUserName {
		t.Fatalf("the name is root but the uid is %d", identity.uid)
	}
}
