package applets_test

import (
	"strings"
	"testing"
)

// find -user and -group take a NAME, as ls -l prints it in its owner and group columns, in any
// case, as Windows takes an account's name, or a number; -uid and -gid a number, with + or -, as
// ls -n prints it. A NAME that is no account is GNU's error, before the walk. They were
// `unrecognized: -user`. What the owner is comes from ls rather than from this test's guess: on
// a runner the files a test makes belong to Administrators, which ls names root.
func TestFind_selectsByOwner(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"tree/f": "x\n"})
	long, _, err := runSmall(t, dir, "", "ls", "-ld", "tree/f")
	numeric, _, errNumeric := runSmall(t, dir, "", "ls", "-ldn", "tree/f")
	if err != nil || errNumeric != nil || len(strings.Fields(long)) < 3 || len(strings.Fields(numeric)) < 3 {
		t.Fatalf("ls -ld tree/f = %q, %v; ls -ldn = %q, %v", long, err, numeric, errNumeric)
	}
	owner, uid := strings.Fields(long)[2], strings.Fields(numeric)[2]
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"tree", "-type", "f", "-user", owner}, "tree/f\n"},
		{[]string{"tree", "-type", "f", "-user", strings.ToUpper(owner)}, "tree/f\n"},
		{[]string{"tree", "-type", "f", "-group", owner}, "tree/f\n"},
		{[]string{"tree", "-type", "f", "-user", uid}, "tree/f\n"},
		{[]string{"tree", "-type", "f", "-uid", uid}, "tree/f\n"},
		{[]string{"tree", "-type", "f", "-gid", uid}, "tree/f\n"},
		{[]string{"tree", "-type", "f", "-uid", "+" + uid}, ""},
		{[]string{"tree", "-type", "f", "!", "-user", owner}, ""},
	} {
		if stdout, stderr, err := runSmall(t, dir, "", "find", test.args...); err != nil || stdout != test.want {
			t.Errorf("find %q = %q, %v (%s); want %q", test.args, stdout, err, stderr, test.want)
		}
	}
	for _, test := range []struct {
		args []string
		says string
	}{
		{[]string{"tree", "-user", "nosuchaccount8417"}, "'nosuchaccount8417' is not the name of a known user"},
		{[]string{"tree", "-group", "nosuchaccount8417"}, "'nosuchaccount8417' is not the name of a known group"},
		{[]string{"tree", "-uid", "x"}, "invalid number 'x'"},
	} {
		stdout, _, err := runSmall(t, dir, "", "find", test.args...)
		if err == nil || !strings.Contains(err.Error(), test.says) || stdout != "" {
			t.Errorf("find %q = %q, %v; want %q and nothing found", test.args, stdout, err, test.says)
		}
	}
}
