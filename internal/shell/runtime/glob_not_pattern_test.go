package runtime_test

import "testing"

// A word with a `[` nothing closes, and no `*` or `?`, is no pattern, so nullglob does not make
// it vanish and failglob does not refuse it, as bash has it: `[` under nullglob was nothing, so
// `[ 1 = 1 ]` ran `1`, and under failglob every `[ ... ]` was `no match: [`. An unquoted
// expansion that comes to such a word is the same. Each answer is bash 5.2's.
func TestGlob_aWordThatIsNoPatternStands(t *testing.T) {
	status, stdout, stderr := runSetScript(t, `shopt -s failglob
echo [ y; echo [a y; echo a[ z
[ 1 = 1 ] && echo test-ok
shopt -u failglob; shopt -s nullglob
echo [a w; x="["; echo $x v; echo *.nomatch u
`)
	if want := "[ y\n[a y\na[ z\ntest-ok\n[a w\n[ v\nu\n"; status != 0 || stdout != want {
		t.Fatalf("got %q/%d, want %q; stderr %q", stdout, status, want, stderr)
	}
}
