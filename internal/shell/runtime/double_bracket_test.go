package runtime_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `[[ ]]`, every expectation measured from bash.
//
// The reason it exists is the first case: inside `[[ ]]` a word is not split, so
// an unquoted variable holding a blank works. With `[` the same line becomes
// `[ a b = a b ]` and is a syntax error, which is why people reach for `[[`.
func TestRuntime_doubleBracket(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   int
	}{
		{
			name:   "an unquoted value with a blank in it",
			script: "x=\"a b\"\n[[ $x == \"a b\" ]]\n", want: 0,
		},
		{
			// And the same line with `[` is the error this exists to avoid.
			name:   "the same test with single brackets is a usage error",
			script: "x=\"a b\"\n[ $x = \"a b\" ]\n", want: 2,
		},
		{name: "the right side is a pattern", script: "[[ abc == a* ]]\n", want: 0},
		{
			// Quoting the pattern makes it a literal string. Measured: bash says
			// false here.
			name: "a quoted pattern is a literal", script: "[[ \"abc\" == \"a*\" ]]\n", want: 1,
		},
		{name: "a regular expression", script: "[[ abc =~ ^a.c$ ]]\n", want: 0},
		{name: "an unanchored regular expression", script: "[[ abc =~ b ]]\n", want: 0},
		{name: "a regular expression that does not match", script: "[[ abc =~ ^b ]]\n", want: 1},
		{name: "not equal, as a pattern", script: "[[ abc != x* ]]\n", want: 0},
		{name: "numeric comparison", script: "[[ 3 -lt 5 ]]\n", want: 0},
		{name: "a unary test", script: "[[ -z \"\" ]]\n", want: 0},
		{
			// The other half of the appeal: an unset variable does not become a
			// missing argument, so this is safe without quoting.
			name: "an unset variable is safe", script: "empty=\n[[ -n $empty ]]\n", want: 1,
		},
		{name: "and", script: "[[ 1 -eq 1 && 2 -eq 2 ]]\n", want: 0},
		{name: "and, short-circuiting to false", script: "[[ 1 -eq 2 && 2 -eq 2 ]]\n", want: 1},
		{name: "or", script: "[[ 1 -eq 2 || 3 -eq 3 ]]\n", want: 0},
		{name: "negation", script: "[[ ! -z x ]]\n", want: 0},
		{name: "parentheses", script: "[[ ( 1 -eq 1 ) ]]\n", want: 0},
		{
			name:   "grouping changes the answer",
			script: "x=5\n[[ $x -gt 3 && ( $x -lt 10 || $x -eq 99 ) ]]\n", want: 0,
		},
		{name: "a lexical comparison", script: "[[ a < b ]]\n", want: 0},
		{
			// The one that proves `<` is not a redirection here. Before the lexer
			// knew about `[[ ]]`, this created a file called `b` and the test
			// passed for the wrong reason.
			name: "a lexical comparison that is false", script: "[[ b < a ]]\n", want: 1,
		},
		{name: "a bare non-empty word is true", script: "[[ x ]]\n", want: 0},
		{name: "a bare empty word is false", script: "[[ \"\" ]]\n", want: 1},
		{
			// 2, not 1: "that was not an expression" has to be distinguishable
			// from "the answer is no".
			name: "a malformed expression is 2", script: "[[ 1 -eq ]]\n", want: 2,
		},
		{name: "a missing closer is 2", script: "[[ 1 -eq 1\n", want: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, _, stderr := runSetScript(t, test.script)

			// Then
			if status != test.want {
				t.Fatalf("%q exited %d, want %d (stderr %q)", test.script, status, test.want, stderr)
			}
		})
	}
}

// `<` and `>` inside `[[ ]]` must not redirect, which is a lexer question rather
// than an evaluation one: the proof is that no file appears.
func TestRuntime_doubleBracketDoesNotRedirect(t *testing.T) {
	// Given
	directory := t.TempDir()
	// The operands are paths inside the directory rather than bare words, and no
	// `cd` is used: a redirection would create the file named on the right, so the
	// directory staying empty is the whole assertion. Both comparisons are true,
	// so a non-zero status would mean something other than the comparison went
	// wrong.
	inside := filepath.ToSlash(directory) + "/"
	script := "[[ " + inside + "a < " + inside + "b ]]\n[[ " + inside + "d > " + inside + "c ]]\n"

	// When
	status, _, stderr := runSetScript(t, script)

	// Then
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("the conditional created %v; `<` and `>` are comparisons inside [[ ]]", names)
	}
}

// The unary tests are `test`'s own, called through one exported entry point --
// two copies of `-f` would drift, and this project has fixed that class of bug
// twice.
func TestRuntime_doubleBracketFileTests(t *testing.T) {
	// Given
	directory := t.TempDir()
	file := filepath.Join(directory, "there.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	slashed := filepath.ToSlash(file)

	tests := []struct {
		name   string
		script string
		want   int
	}{
		{name: "-f on a file", script: "[[ -f " + slashed + " ]]\n", want: 0},
		{name: "-d on a file", script: "[[ -d " + slashed + " ]]\n", want: 1},
		{name: "-e on something absent", script: "[[ -e " + slashed + ".missing ]]\n", want: 1},
		{name: "-s on a non-empty file", script: "[[ -s " + slashed + " ]]\n", want: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, _, stderr := runSetScript(t, test.script)

			// Then
			if status != test.want {
				t.Fatalf("%q exited %d, want %d (stderr %q)", test.script, status, test.want, stderr)
			}
		})
	}
}

// `[[` is only a conditional at the start of a command. As an argument it is an
// ordinary word, or `echo [[` would stop working.
func TestRuntime_doubleBracketOnlyAtACommandStart(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "echo [[ done\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr)
	}
	if strings.TrimSpace(stdout) != "[[ done" {
		t.Fatalf("stdout = %q, want the words printed as written", stdout)
	}
}
