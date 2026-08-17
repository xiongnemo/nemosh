package runtime_test

import (
	"strings"
	"testing"
)

// Brace expansion, every expectation measured from bash.
//
// It is not POSIX -- dash has none of it -- so this is a deliberate extension,
// taken from bash because it is what fingers do.
func TestRuntime_expandsBraces(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "a list", script: "echo {a,b}\n", want: "a b\n"},
		{name: "with a prefix and a suffix", script: "echo pre{a,b}post\n", want: "preapost prebpost\n"},
		{name: "a numeric range", script: "echo {1..5}\n", want: "1 2 3 4 5\n"},
		{name: "a descending range", script: "echo {5..1}\n", want: "5 4 3 2 1\n"},
		{name: "a letter range", script: "echo {a..e}\n", want: "a b c d e\n"},
		{
			// The padding is what makes this useful for filenames: a leading zero
			// on either endpoint pads every value to the wider one.
			name: "a zero-padded range", script: "echo {01..03}\n", want: "01 02 03\n",
		},
		{name: "a range with a step", script: "echo {1..10..3}\n", want: "1 4 7 10\n"},
		{
			// Two groups multiply rather than concatenate.
			name: "two groups are a product", script: "echo {a,b}{1,2}\n", want: "a1 a2 b1 b2\n",
		},
		{name: "three groups", script: "echo a{b,c}d{e,f}\n", want: "abde abdf acde acdf\n"},
		{name: "nesting", script: "echo {a,b{c,d}}\n", want: "a bc bd\n"},
		{
			// No comma and no range, so it is not a group at all. bash prints it
			// unchanged, and so must this -- otherwise `mkdir {}` would become
			// something else.
			name: "no comma and no range", script: "echo {a}\n", want: "{a}\n",
		},
		{name: "empty braces", script: "echo {}\n", want: "{}\n"},
		{
			// Quoted, so the braces are not in an unquoted literal and are never
			// seen as a group. This falls out of the atom model rather than being
			// a special case.
			name: "quoted braces are literal", script: "echo \"{a,b}\"\n", want: "{a,b}\n",
		},
		{name: "escaped braces are literal", script: "echo \\{a,b\\}\n", want: "{a,b}\n"},
		{
			// The fact that decides the whole implementation: brace expansion runs
			// before parameters exist, so the alternatives have to carry the
			// unexpanded parameter with them.
			name: "a parameter inside a group", script: "x=1\necho {$x,2}\n", want: "1 2\n",
		},
		{
			// An unmatched brace leaves the whole word alone, which is what bash
			// does with `echo {a,b`.
			name: "an unmatched brace", script: "echo {a,b\n", want: "{a,b\n",
		},
		{
			// A trailing empty alternative is a word: `{a,}` is two words, `a` and
			// nothing, which echo prints with a trailing blank.
			name: "an empty alternative", script: "echo [{a,}]\n", want: "[a] []\n",
		},
		{name: "a range that is not one", script: "echo {1..x}\n", want: "{1..x}\n"},
		{
			// A group that is not a group does not stop the scan: the second one
			// still expands.
			name: "a non-group before a group", script: "echo {a}{b,c}\n", want: "{a}b {a}c\n",
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

// Brace expansion produces words, so it has to work where words are used -- not
// only as an argument to echo.
func TestRuntime_expandsBracesWhereverWordsGo(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{
			name:   "in a for list",
			script: "for i in {1..3}; do printf '%s' $i; done\necho\n",
			want:   "123\n",
		},
		{
			// The words reach `set --`, so the positional parameters count them.
			name:   "as positional parameters",
			script: "set -- {a,b,c}\necho $#\n",
			want:   "3\n",
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

// A case pattern is not brace-expanded, because there the pattern is the point --
// the same reason pathname expansion is kept away from it.
func TestRuntime_leavesCasePatternsAlone(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "case '{a,b}' in {a,b}) echo matched;; *) echo no;; esac\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr)
	}
	if !strings.Contains(stdout, "matched") {
		t.Fatalf("stdout = %q, want the literal pattern to have matched", stdout)
	}
}
