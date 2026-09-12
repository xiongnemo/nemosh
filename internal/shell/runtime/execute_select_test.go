package runtime_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// `select`, the last of the 46 constructs the v1.1 probe measured and the only one it
// found missing. Every expectation below was measured against bash on the same input.
//
// The menu and the prompt go to **stderr**, which is what makes `x=$(select ...)` capture
// the answer instead of the list -- so these assertions keep the two streams apart rather
// than reading a combined transcript.

// runSelect drives a script with a canned answer stream, and answers the two streams
// separately because which one the menu lands on is half of what is being tested.
func runSelect(t *testing.T, answers, script string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{
		Stdin:  strings.NewReader(answers),
		Stdout: &stdout,
		Stderr: &stderr,
	})
	return rt.RunScript(context.Background(), script), stdout.String(), stderr.String()
}

func TestSelect_readsAChoice(t *testing.T) {
	for _, test := range []struct {
		name       string
		answers    string
		script     string
		wantStdout string
		wantStderr string
	}{
		{
			name: "a number chooses the word", answers: "2\n",
			script:     "select x in a b c; do echo \"[$x]\"; break; done\n",
			wantStdout: "[b]\n",
			// The menu is numbered from one and goes to stderr, and the prompt is PS3's
			// default with no newline after it.
			wantStderr: "1) a\n2) b\n3) c\n#? ",
		},
		{
			// Out of range is not an error: the name goes empty and the body still
			// runs, which is what makes the conventional `case $x in *) invalid`
			// inside the loop work at all.
			name: "out of range gives an empty name", answers: "9\n",
			script:     "select x in a b c; do echo \"[$x]\"; break; done\n",
			wantStdout: "[]\n",
			wantStderr: "1) a\n2) b\n3) c\n#? ",
		},
		{
			name: "not a number gives an empty name", answers: "zz\n",
			script:     "select x in a b; do echo \"[$x]\"; break; done\n",
			wantStdout: "[]\n",
			wantStderr: "1) a\n2) b\n#? ",
		},
		{
			// REPLY is the line as typed, not the chosen word, and it is set even when
			// the answer was not a valid number -- a script validating its own input
			// needs the raw text.
			name: "REPLY is the raw line", answers: "zz\n",
			script:     "select x in a b; do echo \"[$REPLY]\"; break; done\n",
			wantStdout: "[zz]\n",
			wantStderr: "1) a\n2) b\n#? ",
		},
		{
			// A blank answer reprints the menu and does *not* run the body.
			name: "an empty line reprints the menu", answers: "\n1\n",
			script:     "select x in a b; do echo \"[$x]\"; break; done\n",
			wantStdout: "[a]\n",
			wantStderr: "1) a\n2) b\n#? 1) a\n2) b\n#? ",
		},
		{
			// End of input ends the loop, which is what Ctrl-D does interactively.
			name: "end of input ends it", answers: "",
			script:     "select x in a b; do echo ran; done\necho after\n",
			wantStdout: "after\n",
			wantStderr: "1) a\n2) b\n#? ",
		},
		{
			// An empty list asks nothing, as an empty `for` iterates over nothing.
			name: "an empty list runs nothing", answers: "",
			script:     "select x in; do echo no; done\necho after\n",
			wantStdout: "after\n",
			wantStderr: "",
		},
		{
			name: "PS3 is the prompt", answers: "1\n",
			script:     "PS3=\"pick> \"\nselect x in a; do break; done\n",
			wantStdout: "",
			wantStderr: "1) a\npick> ",
		},
		{
			// No list is the positional parameters, the same default `for` has.
			name: "no list means the arguments", answers: "2\n",
			script:     "set -- p q\nselect x; do echo \"[$x]\"; break; done\n",
			wantStdout: "[q]\n",
			wantStderr: "1) p\n2) q\n#? ",
		},
		{
			// A word that expands to several is several entries; one holding a blank
			// is one entry, which is why the menu is built from the expansion rather
			// than from the source text.
			name: "a quoted word stays one entry", answers: "1\n",
			script:     "select x in \"two words\" b; do echo \"[$x]\"; break; done\n",
			wantStdout: "[two words]\n",
			wantStderr: "1) two words\n2) b\n#? ",
		},
		{
			// The numbers are right-aligned, so a list of more than nine still lines
			// up. Ten entries is the smallest list that shows it.
			name: "numbers align past nine", answers: "10\n",
			script:     "select x in 1 2 3 4 5 6 7 8 9 10; do echo \"[$x]\"; break; done\n",
			wantStdout: "[10]\n",
			wantStderr: " 1) 1\n 2) 2\n 3) 3\n 4) 4\n 5) 5\n 6) 6\n 7) 7\n 8) 8\n 9) 9\n10) 10\n#? ",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			status, stdout, stderr := runSelect(t, test.answers, test.script)
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
			if stdout != test.wantStdout {
				t.Errorf("stdout\n  got  %q\n  want %q", stdout, test.wantStdout)
			}
			if stderr != test.wantStderr {
				t.Errorf("stderr\n  got  %q\n  want %q", stderr, test.wantStderr)
			}
		})
	}
}

// The loop keywords work inside it, including the level form -- which is the part that
// would be easy to get wrong by writing a bespoke loop instead of following the one every
// other loop here uses.
func TestSelect_breakAndContinue(t *testing.T) {
	for _, test := range []struct {
		name    string
		answers string
		script  string
		want    string
	}{
		{
			name: "continue asks again", answers: "9\n2\n",
			script: "select x in a b; do [ -z \"$x\" ] && continue; echo \"[$x]\"; break; done\n",
			want:   "[b]\n",
		},
		{
			// `break 2` leaves the select *and* the loop around it, so `after` prints
			// once rather than twice.
			name: "break 2 leaves the loop outside", answers: "1\n",
			script: "for i in 1 2; do select x in a; do break 2; done; done\necho after\n",
			want:   "after\n",
		},
		{
			name: "the body's status is the loop's", answers: "1\n",
			script: "select x in a; do false; break; done\necho $?\n",
			want:   "0\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			status, stdout, stderr := runSelect(t, test.answers, test.script)
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
			if stdout != test.want {
				t.Fatalf("%s\n  got  %q\n  want %q", test.script, stdout, test.want)
			}
		})
	}
}

// The menu is on stderr, so redirecting it away leaves only what the body printed. This
// is the property that makes select usable in a substitution, and it is asserted on its
// own because every case above would still pass if both streams were the same buffer.
func TestSelect_menuIsSeparableFromOutput(t *testing.T) {
	status, stdout, stderr := runSelect(t, "1\n",
		"select x in a b; do echo \"[$x]\"; break; done 2>/dev/null\n")
	if status != 0 {
		t.Fatalf("status = %d", status)
	}
	if stdout != "[a]\n" {
		t.Errorf("stdout = %q, want %q", stdout, "[a]\n")
	}
	if stderr != "" {
		t.Errorf("the menu survived 2>/dev/null: %q", stderr)
	}
}
