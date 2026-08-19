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

// The operators in a `case` pattern and inside `[[ ]]`, which is where they are usually
// written. Eight scans had to learn that a `(` after one of `?*+@!` belongs to the pattern:
// the logical-line scanner, the `;` separator, the continuation scan, the group extractor,
// the deferred scan that refuses parentheses, the one that cuts `pattern) body`, the one
// that splits alternatives on `|`, and the lexer -- which read the `|` inside the group as
// a pipe and turned one word into three.
//
// Measured against `bash -O extglob`.
func TestExtendedPattern_inCaseAndDoubleBracket(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "at in a case", script: "case abc in @(abc|xyz)) echo at ;; esac\n", want: "at\n"},
		{
			// The shape a script actually writes.
			name:   "alternatives of globs",
			script: "case file.jpg in @(*.jpg|*.png)) echo image ;; esac\n", want: "image\n",
		},
		{name: "question in a case", script: "case a in ?(a)) echo opt ;; esac\n", want: "opt\n"},
		{name: "plus in a case", script: "case aab in +(a)b) echo plus ;; esac\n", want: "plus\n"},
		{name: "plus with alternatives", script: "case aab in +(a|ab)) echo backtrack ;; esac\n", want: "backtrack\n"},
		{name: "negation in a case", script: "case b in !(a)) echo neg ;; esac\n", want: "neg\n"},
		{
			name:   "negation not matching falls to the star",
			script: "case a in !(a)) echo no ;; *) echo star ;; esac\n", want: "star\n",
		},
		{name: "at inside double brackets", script: "[[ abc == @(abc|x) ]] && echo db\n", want: "db\n"},
		{name: "plus inside double brackets", script: "[[ abc == +(a|b|c) ]] && echo plus\n", want: "plus\n"},
		{
			name:   "negation inside double brackets",
			script: "[[ b == !(a) ]] && echo neg\n", want: "neg\n",
		},
		{name: "a group written across arms", script: "case png in @(jpg|png)) echo m ;; esac\n", want: "m\n"},
		// The forms these eight scans exist for, which must be untouched.
		{name: "an ordinary alternative list", script: "case b in a|b) echo alt ;; esac\n", want: "alt\n"},
		{name: "an ordinary pattern", script: "case abc in a*) echo plain ;; esac\n", want: "plain\n"},
		{name: "the optional open paren", script: "case a in (a) echo p ;; esac\n", want: "p\n"},
		{name: "a subshell", script: "(echo sub)\n", want: "sub\n"},
		{name: "a pipeline", script: "echo a | cat\n", want: "a\n"},
		{name: "a pipe in a quoted word", script: "echo 'a|b'\n", want: "a|b\n"},
		{name: "an arithmetic command", script: "i=0\n((i++))\necho $i\n", want: "1\n"},
		{name: "a counted for", script: "for ((i=0;i<2;i++)); do printf %s $i; done\necho\n", want: "01\n"},
		{name: "an array assignment", script: "a=(x y)\necho ${a[1]}\n", want: "y\n"},
		{name: "a nested condition", script: "[[ ( a == a ) ]] && echo nested\n", want: "nested\n"},
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
