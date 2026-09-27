package runtime_test

import "testing"

// `shopt -s xpg_echo` makes echo expand backslash escapes without -e, as bash's echo does, and
// -E still turns them off. busybox has no shopt, and its echo expands them only with -e.
func TestShopt_xpgEcho(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t,
		"echo 'a\\tb'\nshopt -s xpg_echo\necho 'a\\tb'\necho -E 'a\\tb'\necho -n 'c\\n'\necho end\n")

	// Then
	if want := "a\\tb\na\tb\na\\tb\nc\nend\n"; stdout != want || status != 0 {
		t.Fatalf("got %q/%d, want %q/0; stderr = %q", stdout, status, want, stderr)
	}
}
