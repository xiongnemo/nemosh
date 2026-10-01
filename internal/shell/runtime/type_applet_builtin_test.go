package runtime

import "testing"

// time, timeout and which are builtins here and applets in busybox, which calls them builtin
// applets; so does type now, and command -V with it. They were "a shell builtin". To type -t
// they are builtins either way.
func TestType_callsTheBuiltinsBusyboxHasAsAppletsBuiltinApplets(t *testing.T) {
	stdout, stderr, _ := runKill(t, "type time timeout which cd\ncommand -V which\ntype -t which\n")
	want := "time is a builtin applet\ntimeout is a builtin applet\nwhich is a builtin applet\ncd is a shell builtin\nwhich is a builtin applet\nbuiltin\n"
	if stdout != want || stderr != "" {
		t.Fatalf("stdout %q, stderr %q; want %q", stdout, stderr, want)
	}
}
