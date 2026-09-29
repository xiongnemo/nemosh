package runtime_test

import "testing"

// builtin takes -- as the end of its options, as bash does (busybox has no builtin): `builtin --`
// does nothing and `builtin -- echo hi` runs echo. It was refused as a builtin named --.
func TestBuiltin_takesDashDash(t *testing.T) {
	script := "builtin --; echo a=$?\nbuiltin -- false; echo b=$?\nbuiltin -- echo hi\n"
	if stdout, status := runScriptCapturing(script); stdout != "a=0\nb=1\nhi\n" || status != 0 {
		t.Fatalf("got %q/%d, want %q/0, as bash answers", stdout, status, "a=0\nb=1\nhi\n")
	}
}
