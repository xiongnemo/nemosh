package runtime_test

import "testing"

// `type` answers in the order dispatch tries a name, and with bash's options. Every answer is
// bash's, measured, except the special-builtin case, where this shell's own dispatch -- the
// POSIX one -- is what `type` has to describe.
func TestType_answersAsDispatchWould(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
	}{
		{name: "-t names the kind", script: "f() { :; }\ntype -t cd f if nope\necho \"st=$?\"\n", want: "builtin\nfunction\nkeyword\nst=1\n"},
		{name: "a keyword", script: "type if\n", want: "if is a shell keyword\n"},
		// Dispatch runs a function before an ordinary builtin, so type has to say so; it
		// used to call cd a builtin while running cd ran the function.
		{name: "a function shadows a builtin", script: "cd() { :; }\ntype -t cd\n", want: "function\n"},
		// ...but not a special builtin, which POSIX puts first. bash's default mode
		// differs here; its POSIX mode does not.
		{name: "a special builtin is not shadowed", script: "break() { :; }\ntype -t break\n", want: "builtin\n"},
		{name: "-p of a builtin is nothing", script: "type -p cd\necho \"st=$?\"\n", want: "st=0\n"},
		{name: "an applet runs in the shell", script: "type -t cat\n", want: "builtin\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if stdout, _ := runScriptCapturing(test.script); stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}
