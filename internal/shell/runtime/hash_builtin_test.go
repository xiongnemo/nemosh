package runtime_test

import "testing"

// hash answers whether a name can be run, as busybox's does: `hash git || die` is what a
// script uses it for, and it was refused with 126. Nothing is remembered, since lookup here
// is never cached, so `hash` lists nothing and `hash -r` has nothing to forget.
func TestHash(t *testing.T) {
	tests := []struct {
		name, script, stdout, stderr string
	}{
		{name: "nothing is remembered", script: "hash\necho \"st=$?\"\nhash -r\necho \"st=$?\"\n", stdout: "st=0\nst=0\n"},
		{
			name:   "a builtin, an applet and a function are there",
			script: "f() { :; }\nhash cd ls f\necho \"st=$?\"\n",
			stdout: "st=0\n",
		},
		{
			name:   "a name that is not is 1",
			script: "hash ls nosuch-zz\necho \"st=$?\"\n",
			stdout: "st=1\n", stderr: "hash: nosuch-zz: not found\n",
		},
		{name: "an alias is not a command", script: "alias ll=ls\nhash ll\necho \"st=$?\"\n", stdout: "st=1\n", stderr: "hash: ll: not found\n"},
		{name: "a path is passed over", script: "hash ./nosuch\necho \"st=$?\"\n", stdout: "st=0\n"},
		{name: "an option it has not got", script: "hash -x\necho \"st=$?\"\n", stdout: "st=2\n", stderr: "hash: illegal option -x\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if stdout != test.stdout || stderr != test.stderr || status != 0 {
				t.Fatalf("got %q / %q / %d, want %q / %q / 0", stdout, stderr, status, test.stdout, test.stderr)
			}
		})
	}
}
