package runtime_test

import (
	"strings"
	"testing"
)

// `for ((init; condition; step))` reported `expected: for name in words`, because the
// only `for` this parser knew was the word-list one. Together with `((expr))`, which
// parsed as nested subshells, that ruled out every loop written with a counter.
func TestArithmeticFor_runsACountedLoop(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "counting up", script: "for ((i=0;i<3;i++)); do printf %s $i; done\necho\n", want: "012\n"},
		{name: "with blanks", script: "for ((i = 0; i < 3; i++)); do printf %s $i; done\necho\n", want: "012\n"},
		{name: "counting down", script: "for ((i=3;i>0;i--)); do printf %s $i; done\necho\n", want: "321\n"},
		{name: "a step of two", script: "for ((i=0;i<6;i+=2)); do printf %s $i; done\necho\n", want: "024\n"},
		{
			// Any part may be empty.
			name: "no initialiser", script: "i=0\nfor ((;i<3;i++)); do printf %s $i; done\necho\n", want: "012\n",
		},
		{
			name:   "no step",
			script: "for ((i=0;i<3;)); do printf %s $i; i=$((i+1)); done\necho\n", want: "012\n",
		},
		{
			// An empty condition is true, which is what makes `for ((;;))` a loop
			// forever rather than one that never runs.
			name:   "no condition, left by a break",
			script: "i=0\nfor ((;;)); do i=$((i+1)); if ((i==3)); then break; fi; done\necho $i\n", want: "3\n",
		},
		{
			name:   "a condition false at the start runs nothing",
			script: "for ((i=5;i<3;i++)); do echo never; done\necho done\n", want: "done\n",
		},
		{name: "the counter survives the loop", script: "for ((i=0;i<3;i++)); do :; done\necho $i\n", want: "3\n"},
		{
			name:   "nested",
			script: "for ((i=0;i<2;i++)); do for ((j=0;j<2;j++)); do printf '%s%s ' $i $j; done; done\necho\n",
			want:   "00 01 10 11 \n",
		},
		{
			name:   "the body sees the variable",
			script: "for ((i=1;i<=3;i++)); do printf '%s' $((i*i)); done\necho\n", want: "149\n",
		},
		{
			// A part may hold parentheses of its own, so the split on `;` has to be
			// depth-aware.
			name:   "parentheses inside a part",
			script: "for ((i=(1+1);i<4;i++)); do printf %s $i; done\necho\n", want: "23\n",
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

// break and continue have to work, and the step has to run on a continue -- which is
// what C does and what stops `continue` from spinning the loop forever.
func TestArithmeticFor_honoursBreakAndContinue(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{
			name:   "break",
			script: "for ((i=0;i<5;i++)); do if ((i==2)); then break; fi; printf %s $i; done\necho\n",
			want:   "01\n",
		},
		{
			// 023: the step ran on the iteration that continued, so the loop moved on.
			name:   "continue still runs the step",
			script: "for ((i=0;i<4;i++)); do if ((i==1)); then continue; fi; printf %s $i; done\necho\n",
			want:   "023\n",
		},
		{
			name:   "break 2 from inside",
			script: "for ((i=0;i<3;i++)); do for ((j=0;j<3;j++)); do break 2; done; echo inner; done\necho out\n",
			want:   "out\n",
		},
		{
			name:   "continue 2 from inside",
			script: "for ((i=0;i<2;i++)); do for ((j=0;j<3;j++)); do continue 2; done; echo never; done\necho out\n",
			want:   "out\n",
		},
		{
			// Mixed with the word-list form, which shares the level counter.
			name:   "break 2 out of a word-list loop",
			script: "for w in a b; do for ((i=0;i<3;i++)); do break 2; done; echo inner; done\necho out\n",
			want:   "out\n",
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

// The word-list form is the one POSIX has and must be untouched.
func TestArithmeticFor_leavesTheWordListFormAlone(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "a word list", script: "for w in a b c; do printf %s $w; done\necho\n", want: "abc\n"},
		{name: "one word", script: "for w in only; do echo $w; done\n", want: "only\n"},
		{
			name: "a list from a substitution", script: "for w in $(echo x y); do printf %s $w; done\necho\n",
			want: "xy\n",
		},
		{name: "an array", script: "a=(p q)\nfor w in \"${a[@]}\"; do printf %s $w; done\necho\n", want: "pq\n"},
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

// A header that is none of the forms has to say what the forms are, rather than name
// only the one it happened to check first.
//
// The example was `for broken; do` until `for name` -- POSIX's loop over the positional
// parameters -- was implemented, which made that input legal. `for a b c` is the
// malformed one now: three words and no `in`.
func TestArithmeticFor_saysWhatItExpected(t *testing.T) {
	// When
	_, _, stderr := runSetScript(t, "for a b c; do echo x; done\n")

	// Then
	if !strings.Contains(stderr, "for ((init; condition; step))") {
		t.Fatalf("stderr = %q, want it to mention both forms", stderr)
	}
}

// POSIX 2.9.4.3 allows an open parenthesis in front of a case pattern. It became a
// pattern that began with a literal parenthesis, matched nothing, and the arm
// silently never ran -- which is how a `case` inside `$( )`, written that way to keep
// a reader's paren matcher balanced, quietly did nothing.
func TestCasePattern_acceptsTheOptionalOpenParenthesis(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "one arm", script: "case a in (a) echo matched ;; esac\n", want: "matched\n"},
		{name: "the second arm", script: "case b in (a) echo no ;; (b) echo yes ;; esac\n", want: "yes\n"},
		{name: "alternatives", script: "case b in (a|b) echo alt ;; esac\n", want: "alt\n"},
		{name: "a glob", script: "case abc in (a*) echo glob ;; esac\n", want: "glob\n"},
		{name: "the default arm", script: "case z in (a) echo no ;; (*) echo default ;; esac\n", want: "default\n"},
		{name: "mixed with the bare form", script: "case b in a) echo no ;; (b) echo yes ;; esac\n", want: "yes\n"},
		// The forms that were already right.
		{name: "the bare form", script: "case a in a) echo plain ;; esac\n", want: "plain\n"},
		{
			// A pattern that really is meant to match a parenthesis still can, by
			// quoting it -- which is the only way it could ever have been written.
			name: "a quoted parenthesis is still a pattern", script: "case \"(x\" in \"(\"*) echo literal ;; esac\n",
			want: "literal\n",
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
