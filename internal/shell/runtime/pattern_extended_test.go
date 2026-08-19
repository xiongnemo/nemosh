package runtime_test

import "testing"

// The extended pattern operators. `${x%%+([0-9])}` and its relatives were a literal match
// against those characters; now they match what they mean.
//
// Every expectation measured against `bash -O extglob`.
func TestExtendedPattern_matchesTheOperators(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "plus strips a run", script: "x=abc123\necho \"${x%%+([0-9])}\"\n", want: "abc\n"},
		{name: "at exactly one", script: "x=abc\necho \"${x%%@(bc)}\"\n", want: "a\n"},
		{name: "plus from the front", script: "x=aab\necho \"${x##+(a)}\"\n", want: "b\n"},
		{name: "at in a replacement", script: "x=abc\necho \"${x/@(b|z)/-}\"\n", want: "a-c\n"},
		{name: "question is optional", script: "x=ab\necho \"${x%%?(b)}\"\n", want: "a\n"},
		{name: "star is zero or more", script: "x=aaab\necho \"${x%%*(a)b}\"\n", want: "\n"},
		{
			// The alternatives are tried at every position, which a greedy loop would get
			// wrong: it would take `a`, then `a`, and have `b` left with no way back.
			name: "alternatives backtrack", script: "x=aab\necho \"${x%%+(a|ab)}\"\n", want: "\n",
		},
		{name: "a nested group", script: "x=abc\necho \"${x%%@(a@(b|x)c)}\"\n", want: "\n"},
		{name: "an alternative of globs", script: "x=file.jpg\necho \"${x%%@(*.jpg|*.png)}\"\n", want: "\n"},
		{
			// A group that matches nothing leaves the value alone, as any pattern does.
			name: "no match leaves it alone", script: "x=abc\necho \"${x%%+([0-9])}\"\n", want: "abc\n",
		},
		{name: "negation", script: "x=b\necho \"${x%%!(a)}\"\n", want: "\n"},
		// Ordinary patterns, unchanged: they take the cheaper matcher.
		{name: "a star", script: "x=abc\necho \"${x%%b*}\"\n", want: "a\n"},
		{name: "a bracket", script: "x=abc\necho \"${x%%[bc]*}\"\n", want: "a\n"},
		{name: "a question mark", script: "x=abc\necho \"${x%%?}\"\n", want: "ab\n"},
		{
			// An unmatched parenthesis is an ordinary character, as an unmatched bracket
			// is in POSIX.
			name: "an unclosed group is literal", script: "x='a@(b'\necho \"${x%%@(b}\"\n", want: "a\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
			if stdout != test.want {
				t.Fatalf("%q printed %q, want %q", test.script, stdout, test.want)
			}
		})
	}
}

// `shopt -s extglob` sits near the top of a great many scripts and was refused by name.
// It succeeds now, and cannot be turned off -- the matcher recognises the operators whether
// or not it is asked to, so accepting `-u` and going on matching them would be a lie.
func TestExtendedPattern_shoptAcceptsIt(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
		status int
	}{
		{name: "setting it succeeds", script: "shopt -s extglob && echo ok\n", want: "ok\n", status: 0},
		{name: "it reports on", script: "shopt -q extglob && echo on\n", want: "on\n", status: 0},
		{name: "unsetting is refused", script: "shopt -u extglob\n", want: "", status: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, _ := runSetScript(t, test.script)

			// Then
			if status != test.status {
				t.Fatalf("status = %d, want %d", status, test.status)
			}
			if stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}
