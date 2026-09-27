package runtime_test

import "testing"

// `shopt -s shift_verbose` makes a shift past the last positional parameter say so. Both
// references say nothing without it, and the status is 1 either way. The words are bash's;
// busybox has no shopt.
func TestShopt_shiftVerbose(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t,
		"set -- a\nshift 5\necho \"quiet st=$?\"\nshopt -s shift_verbose\nshift 5\necho \"st=$?\"\nshift\nshift\necho \"st=$? $#\"\n")

	// Then
	if want := "quiet st=1\nst=1\nst=1 0\n"; stdout != want || status != 0 {
		t.Fatalf("got %q/%d, want %q/0; stderr = %q", stdout, status, want, stderr)
	}
	if want := "shift: 5: shift count out of range\nshift: shift count out of range\n"; stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
}
