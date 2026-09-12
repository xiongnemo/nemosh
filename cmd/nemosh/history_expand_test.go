package main

import (
	"bytes"
	"strings"
	"testing"
)

// History expansion, measured against bash rather than recalled.
//
// The rules that carry it are quoting rules, and they are asymmetric in a way that looks
// like a bug until you rely on it: **single quotes protect and double quotes do not**.
// Every other case here is a variation on which `!` is a reference and which is text.

func TestExpandHistory(t *testing.T) {
	history := []string{"echo one two three", "ls -la /tmp", "grep needle haystack"}
	for _, test := range []struct {
		name    string
		line    string
		want    string
		changed bool
	}{
		// The events.
		{name: "!! is the previous line", line: "!!", want: "grep needle haystack", changed: true},
		{name: "!! in the middle", line: "sudo !!", want: "sudo grep needle haystack", changed: true},
		{name: "!n counts as history does", line: "!1", want: "echo one two three", changed: true},
		{name: "!-n counts back", line: "!-2", want: "ls -la /tmp", changed: true},
		{name: "!-1 is the previous", line: "!-1", want: "grep needle haystack", changed: true},
		{name: "!prefix finds the newest", line: "!ls", want: "ls -la /tmp", changed: true},
		{name: "!prefix skips older matches", line: "!echo", want: "echo one two three", changed: true},
		{name: "!?text? searches anywhere", line: "!?needle?", want: "grep needle haystack", changed: true},

		// The word designators, on the previous line.
		{name: "!$ is the last word", line: "vim !$", want: "vim haystack", changed: true},
		{name: "!^ is the first argument", line: "cat !^", want: "cat needle", changed: true},
		{name: "!* is every argument", line: "wc !*", want: "wc needle haystack", changed: true},

		// Quoting. The asymmetry is the point.
		{name: "single quotes protect", line: "echo '!!'", want: "echo '!!'", changed: false},
		{name: "double quotes do not", line: `echo "!!"`, want: `echo "grep needle haystack"`, changed: true},
		{name: "a backslash escapes", line: `echo \!\!`, want: "echo !!", changed: true},
		{name: "a quote inside the other kind", line: `echo "it's !$"`, want: `echo "it's haystack"`, changed: true},

		// A `!` that begins nothing is text, which is what keeps `!=` working.
		{name: "not-equal survives", line: "[ x != y ]", want: "[ x != y ]", changed: false},
		{name: "a trailing bang is text", line: "echo hi!", want: "echo hi!", changed: false},
		{name: "a bang before a blank is text", line: "echo ! there", want: "echo ! there", changed: false},

		// A reference ends at the first character that could not begin a command.
		{name: "a reference ends at a pipe", line: "!ls|wc", want: "ls -la /tmp|wc", changed: true},
		{name: "a reference ends at a semicolon", line: "!ls; echo after", want: "ls -la /tmp; echo after", changed: true},

		// Nothing to do.
		{name: "an ordinary line is untouched", line: "echo hello", want: "echo hello", changed: false},
		{name: "an empty line", line: "", want: "", changed: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, changed, err := expandHistory(test.line, history)
			if err != nil {
				t.Fatalf("%q: %v", test.line, err)
			}
			if got != test.want {
				t.Errorf("%q\n  got  %q\n  want %q", test.line, got, test.want)
			}
			if changed != test.changed {
				t.Errorf("%q reported changed=%v, want %v", test.line, changed, test.changed)
			}
		})
	}
}

// `^old^new` repeats the previous line with one replacement, and only at the very start of
// a line -- which is what makes it unambiguous against a `^` used for anything else.
func TestExpandHistory_quickSubstitution(t *testing.T) {
	history := []string{"echo one two three"}
	for _, test := range []struct {
		name string
		line string
		want string
	}{
		{name: "replaces the first occurrence", line: "^two^TWO", want: "echo one TWO three"},
		{name: "a trailing caret is allowed", line: "^two^TWO^", want: "echo one TWO three"},
		{name: "deleting is replacing with nothing", line: "^ two^", want: "echo one three"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, changed, err := expandHistory(test.line, history)
			if err != nil || !changed {
				t.Fatalf("%q: changed=%v err=%v", test.line, changed, err)
			}
			if got != test.want {
				t.Fatalf("%q\n  got  %q\n  want %q", test.line, got, test.want)
			}
		})
	}
	// A caret that is not at the start is text, so a redirection or a regular
	// expression is untouched.
	for _, line := range []string{"grep '^root' /etc/passwd", "echo a^b^c"} {
		got, changed, err := expandHistory(line, history)
		if err != nil || changed || got != line {
			t.Errorf("%q was rewritten to %q (changed=%v, err=%v)", line, got, changed, err)
		}
	}
}

// A reference that resolves to nothing is an error and the line does not run. Leaving the
// text as typed would send `!vim` to PATH as a command name.
func TestExpandHistory_refusesWhatItCannotFind(t *testing.T) {
	for _, test := range []struct {
		name    string
		history []string
		line    string
		says    string
	}{
		{name: "no history at all", history: nil, line: "!!", says: "event not found"},
		{name: "no such prefix", history: []string{"echo hi"}, line: "!nosuch", says: "!nosuch: event not found"},
		{name: "a number past the end", history: []string{"echo hi"}, line: "!9", says: "!9: event not found"},
		{name: "nothing contains it", history: []string{"echo hi"}, line: "!?zzz?", says: "event not found"},
		{name: "substitution with no match", history: []string{"echo hi"}, line: "^zzz^yyy", says: "substitution failed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := expandHistory(test.line, test.history)
			if err == nil {
				t.Fatalf("%q was accepted", test.line)
			}
			if !strings.Contains(err.Error(), test.says) {
				t.Fatalf("%q said %q, which does not contain %q", test.line, err, test.says)
			}
		})
	}
}

// fakeHistory is a history source that is not a shell.
type fakeHistory []string

func (f fakeHistory) HistoryEntries() []string { return f }

// The wiring: an expansion is echoed on stderr before it runs, an unchanged line is not,
// and the line's own terminator survives -- the plain loop accumulates lines with their
// newlines and the edited one does not, so both have to come back as they went in.
func TestApplyHistoryExpansion(t *testing.T) {
	for _, test := range []struct {
		name       string
		line       string
		wantLine   string
		wantStderr string
		runnable   bool
	}{
		{
			name: "an expansion is echoed", line: "!!\n",
			wantLine: "echo hi\n", wantStderr: "echo hi\n", runnable: true,
		},
		{
			// No echo for a line that did not change, or every command would print
			// itself twice.
			name: "an ordinary line is silent", line: "echo there\n",
			wantLine: "echo there\n", wantStderr: "", runnable: true,
		},
		{
			name: "a line with no terminator keeps having none", line: "!!",
			wantLine: "echo hi", wantStderr: "echo hi\n", runnable: true,
		},
		{
			name: "a failure is reported and the line does not run", line: "!nosuch\n",
			wantLine: "", wantStderr: "nemosh: !nosuch: event not found\n", runnable: false,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stderr bytes.Buffer
			line, runnable := applyHistoryExpansion(fakeHistory{"echo hi"}, &stderr, test.line)
			if runnable != test.runnable {
				t.Fatalf("runnable = %v, want %v", runnable, test.runnable)
			}
			if line != test.wantLine {
				t.Errorf("line\n  got  %q\n  want %q", line, test.wantLine)
			}
			if stderr.String() != test.wantStderr {
				t.Errorf("stderr\n  got  %q\n  want %q", stderr.String(), test.wantStderr)
			}
		})
	}
}
