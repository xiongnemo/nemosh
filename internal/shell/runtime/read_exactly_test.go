package runtime_test

import "testing"

// read -N reads exactly its count, a newline included, and gives what it read to the first
// name whole: not split on IFS, and not trimmed of it. It was split as -n's is, so `read -N 5
// a b c` over "a b c" gave a, b and c. The answers are bash 5.3's; busybox's read has no -N.
func TestRuntime_readExactlyGivesTheFirstNameAll(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{"printf 'a b c\\n' | { read -N 5 A B C; echo \"'$A' '$B' '$C'\"; }", "'a b c' '' ''\n"},
		{"printf 'a b c\\n' | { read -N 4 A B C; echo \"'$A' '$B' '$C'\"; }", "'a b ' '' ''\n"},
		{"printf 'ab\\ncd\\n' | { read -N 4 x; echo \"[$x]\"; }", "[ab\nc]\n"},
		{"printf '  a b' | { read -N 9 x y; echo \"[$x] [$y] $?\"; }", "[  a b] [] 1\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}
