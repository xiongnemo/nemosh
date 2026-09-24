package runtime_test

import "testing"

// `a |& b` is bash's `a 2>&1 | b`, wherever a pipe can go: after a command, after a
// compound's closer, before a compound, at the end of a line. It was a pipe followed by a
// background `&`, and a syntax error. Every answer is bash's, measured.
func TestPipeStderr_carriesBothStreams(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
	}{
		{name: "after a group", script: "{ echo out; echo err >&2; } |& cat\n", want: "out\nerr\n"},
		{name: "after a function call", script: "f() { echo o; echo e >&2; }\nf |& wc -l | tr -d ' '\n", want: "2\n"},
		{name: "twice", script: "echo a |& cat |& cat\n", want: "a\n"},
		{name: "after a closer", script: "for i in 1; do echo loop-err >&2; done |& cat\n", want: "loop-err\n"},
		{name: "after a closer and a redirection", script: "while read l; do echo \"$l\" >&2; done <<< x |& cat\n", want: "x\n"},
		{name: "into a compound", script: "echo e |& while read l; do echo \"[$l]\"; done\n", want: "[e]\n"},
		{name: "at the end of a line", script: "echo ok |&\ncat\n", want: "ok\n"},
		{name: "quoted, it is text", script: "echo '|&' |& cat\n", want: "|&\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if stdout, _ := runScriptCapturing(test.script); stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}
