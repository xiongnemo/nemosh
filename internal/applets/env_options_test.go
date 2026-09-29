package applets_test

import "testing"

// env takes busybox's `[-i0] [-u NAME]... [-] [NAME=VALUE]...` and its long forms: -u removes a
// name, or sets one written NAME=VALUE as busybox's putenv does, -0 ends each entry printed with
// NUL, and a lone `-` is -i. It took a leading -i alone and refused the rest.
func TestEnv_takesBusyboxsOptions(t *testing.T) {
	view := permuteTestView{cwd: t.TempDir()}
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"A=1", "B=2", "env", "-u", "A"}, "B=2\n"},
		{[]string{"A=1", "B=2", "env", "--unset=A"}, "B=2\n"},
		{[]string{"A=1", "B=2", "env", "--unset", "B"}, "A=1\n"},
		{[]string{"A=1", "env", "-uA=x", "printenv", "A"}, "x\n"},
		{[]string{"-i", "-0", "A=1", "B=2"}, "A=1\x00B=2\x00"},
		{[]string{"--null", "-i", "A=1"}, "A=1\x00"},
		{[]string{"A=0", "env", "-", "B=1"}, "B=1\n"},
		{[]string{"A=0", "env", "--ignore-environment", "B=1"}, "B=1\n"},
	} {
		stdout, stderr, err := runPermuted(t, view, "", append([]string{"env"}, test.args...)...)
		if stdout != test.want || stderr != "" || err != nil {
			t.Errorf("env %q: got %q, %q, %v; want %q", test.args, stdout, stderr, err, test.want)
		}
	}
	if stdout, _, err := runPermuted(t, view, "", "env", "A=1", "env", "-u", "A", "printenv", "A"); stdout != "" || err == nil {
		t.Errorf("printenv of an unset name: %q, %v; want nothing and a failure", stdout, err)
	}
	if _, _, err := runPermuted(t, view, "", "env", "-x"); err == nil {
		t.Error("env -x succeeded")
	}
}
