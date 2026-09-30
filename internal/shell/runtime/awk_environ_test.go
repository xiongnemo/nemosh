package runtime_test

import "testing"

// awk's ENVIRON is the environment the program was given: the shell's exports and the command's
// own assignments, each a string that is a number too when it looks like one. It was not there
// at all, so ENVIRON["X"] was empty for every X. Each answer is busybox-w32's, measured.
func TestRuntime_awkHasTheEnvironmentInENVIRON(t *testing.T) {
	script := "FOO=bar awk 'BEGIN{print ENVIRON[\"FOO\"]}'\n" +
		"export X=x; awk 'BEGIN{print ENVIRON[\"X\"]}'\n" +
		"N=5 awk 'BEGIN{print ENVIRON[\"N\"]+1}'\n" +
		"N=10 awk 'BEGIN{print (ENVIRON[\"N\"] > 9)}'\n" +
		"unset Y; awk 'BEGIN{print (\"Y\" in ENVIRON)}'\n" +
		"FOO=bar awk 'BEGIN{n=0; for (k in ENVIRON) if (k==\"FOO\") n++; print n}'\n"
	want := "bar\nx\n6\n1\n0\n1\n"
	if stdout, status := runScriptCapturing(script); stdout != want || status != 0 {
		t.Errorf("got %q, status %d; want %q", stdout, status, want)
	}
}
