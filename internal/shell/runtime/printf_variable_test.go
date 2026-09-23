package runtime_test

import "testing"

// `printf -v name ...` formats into a variable. printf is an applet, which cannot reach the
// shell's variables, so it took `-v` as its format and printed it. Every answer here is
// bash's, measured -- busybox's printf has no -v.
func TestPrintfV_formatsIntoAVariable(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		stdout string
		status int
	}{
		{name: "a conversion", script: "printf -v out '%05d' 42\necho \"[$out]\"\n", stdout: "[00042]\n"},
		{name: "the format repeats", script: "printf -v out '%s-' a b c\necho \"[$out]\"\n", stdout: "[a-b-c-]\n"},
		{name: "trailing newlines are kept", script: "printf -v out 'a\\nb\\n'\nprintf '%s|' \"$out\"\necho\n", stdout: "a\nb\n|\n"},
		{name: "into an element", script: "a=(x y)\nprintf -v 'a[1]' '%s' Z\necho \"${a[@]}\"\n", stdout: "x Z\n"},
		{name: "past a --", script: "printf -v out -- '%s' dash\necho \"[$out]\"\n", stdout: "[dash]\n"},
		{name: "into a local", script: "f() { local out; printf -v out hi; echo \"in [$out]\"; }\nf\necho \"out [$out]\"\n", stdout: "in [hi]\nout []\n"},
		{name: "a bad number still assigns", script: "printf -v out '%d' abc\necho \"st=$? [$out]\"\n", stdout: "st=1 [0]\n"},
		{name: "no name", script: "printf -v\necho \"st=$?\"\n", stdout: "st=2\n"},
		{name: "no format", script: "printf -v out\necho \"st=$?\"\n", stdout: "st=2\n"},
		{name: "not a name", script: "printf -v 1bad x\necho \"st=$?\"\n", stdout: "st=2\n"},
		// An ordinary utility refusing, so the script goes on -- not a shell error.
		{name: "readonly refuses", script: "readonly R=1\nprintf -v R x\necho \"st=$? R=$R\"\n", stdout: "st=1 R=1\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.stdout || status != test.status {
				t.Fatalf("got %q/%d, want %q/%d", stdout, status, test.stdout, test.status)
			}
		})
	}
}
