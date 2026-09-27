package runtime_test

import "testing"

// In `=~`'s extended regular expression a `{` begins an interval or the expression is
// invalid, status 2, as busybox-w32 and bash both answer; Go's regexp took the brace for a
// literal. `{,m}` is `{0,m}`, as busybox-w32 reads it. A quoted, escaped or bracketed brace
// is a brace.
func TestRuntime_regexBraceBeginsAnInterval(t *testing.T) {
	script := "[[ { =~ { ]]; echo \"1 $?\"\n" +
		"[[ 'a{' =~ a{ ]]; echo \"2 $?\"\n" +
		"[[ 'a{1' =~ a{1 ]]; echo \"3 $?\"\n" +
		"[[ aa =~ a{2} ]]; echo \"4 $?\"\n" +
		"[[ a =~ a{,2} ]]; echo \"5 $?\"\n" +
		"[[ 'a{' =~ a[{] ]]; echo \"6 $?\"\n" +
		"[[ 'a{' =~ 'a{' ]]; echo \"7 $?\"\n" +
		"[[ 'a{' =~ a\\{ ]]; echo \"8 $?\"\n" +
		"[[ 'a}' =~ a} ]]; echo \"9 $?\"\n" +
		"[[ 'ab' =~ [[:alpha:]]{2} ]]; echo \"10 $?\"\n"
	want := "1 2\n2 2\n3 2\n4 0\n5 0\n6 0\n7 0\n8 0\n9 0\n10 0\n"
	if stdout, status := runScriptCapturing(script); stdout != want || status != 0 {
		t.Errorf("got %q/%d, want %q/0, as busybox-w32 answers", stdout, status, want)
	}
}
