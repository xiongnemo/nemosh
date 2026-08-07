package runtime_test

import "testing"

// A brace is the reserved word only in command position; everywhere else it is
// an ordinary character. The previous byte alone cannot tell those apart, so
// this table fixes the rule against bash, dash, and busybox ash, all three of
// which agree on every row below.
//
// The two directions are not symmetric. `{` opens a group when a separator or
// the `)` of a function definition precedes it and a blank follows it, so
// `f(){ echo a; }` is a definition. `}` closes one only when the previous
// non-blank is a separator, so the `}` in `{ echo $((1+2))}; }` is text -- after
// the `))` the scan is still inside a word.
func TestParseScript_treatsABraceAsReservedOnlyInCommandPosition(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		status int
		stdout string
	}{
		{
			name:   "brace joined to a function definition opens the body",
			script: "f(){ echo joined; }\nf\n",
			stdout: "joined\n",
		},
		{
			name:   "brace joined to a definition on one line",
			script: "f(){ echo joined; }; f\n",
			stdout: "joined\n",
		},
		{
			name:   "a spaced definition still works",
			script: "f() { echo spaced; }\nf\n",
			stdout: "spaced\n",
		},
		{
			name:   "arithmetic inside a brace group",
			script: "{ echo $((1+2)); }\n",
			stdout: "3\n",
		},
		{
			name:   "brace after an arithmetic close is text",
			script: "{ echo $((1+2))}; }\n",
			stdout: "3}\n",
		},
		{
			name:   "a closing brace inside a word is text",
			script: "echo a}b\n",
			stdout: "a}b\n",
		},
		{
			name:   "a closing brace after a command word is text",
			script: "echo }\n",
			stdout: "}\n",
		},
		{
			name:   "a closing brace starting an argument is text",
			script: "echo }x\n",
			stdout: "}x\n",
		},
		{
			name:   "an opening brace inside a word is text",
			script: "echo a{b\n",
			stdout: "a{b\n",
		},
		{
			name:   "a brace pair as one argument, the form find -exec uses",
			script: "echo {}\n",
			stdout: "{}\n",
		},
		{
			name:   "braces around text inside a word",
			script: "echo x{1}y\n",
			stdout: "x{1}y\n",
		},
		{
			name:   "a literal brace inside a real group",
			script: "{ echo a}b; }\n",
			stdout: "a}b\n",
		},
		{
			name:   "a real group still runs",
			script: "{ echo one; echo two; }\n",
			stdout: "one\ntwo\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if status != test.status {
				t.Fatalf("status = %d, stderr = %q, want %d", status, stderr, test.status)
			}
			if stdout != test.stdout {
				t.Fatalf("stdout = %q, want %q", stdout, test.stdout)
			}
		})
	}
}

// A brace that really is in command position with nothing to match stays an
// error, and so does a stray `)`, which bash also refuses.
func TestParseScript_stillRejectsABraceInCommandPosition(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
	}{
		{name: "brace joined to command text", script: "{echo bad;}\n"},
		{name: "unmatched close in command position", script: "echo one\n}\n"},
		{name: "stray close paren inside a word", script: "echo a)b\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, _, stderr := runSetScript(t, test.script)

			// Then
			if status != 2 {
				t.Fatalf("status = %d, stderr = %q, want 2", status, stderr)
			}
		})
	}
}
