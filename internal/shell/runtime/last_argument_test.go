package runtime_test

import "testing"

// `$_` is the last argument of the simple command that just finished -- its name when it had
// none, empty after an assignment alone -- and a function's is its own last argument once it
// returns. It was whatever the environment brought in, for the whole session: a path. Every
// answer here is bash's, measured; busybox has no `$_`.
func TestLastArgument_followsEachCommand(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
	}{
		{name: "the last argument", script: "echo a b >/dev/null\necho \"$_\"\n", want: "b\n"},
		{name: "one field, however many words", script: "echo \"one two\" >/dev/null\necho \"$_\"\n", want: "one two\n"},
		{name: "the name when there is nothing else", script: "true\necho \"$_\"\n", want: "true\n"},
		{name: "empty after an assignment", script: "echo a b >/dev/null\nx=1\necho \"[$_]\"\n", want: "[]\n"},
		{name: "a function's own", script: "f() { :; }\nf x y\necho \"$_\"\n", want: "y\n"},
		{name: "the last command inside a loop", script: "for i in 1 2; do :; done\necho \"$_\"\n", want: ":\n"},
		{name: "a subshell sees it", script: "echo x >/dev/null\n(echo \"$_\")\n", want: "x\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if stdout, _ := runScriptCapturing(test.script); stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}
