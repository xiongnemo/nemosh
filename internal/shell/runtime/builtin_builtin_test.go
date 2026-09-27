package runtime_test

import "testing"

// `builtin name` runs the builtin even where a function has the name, as bash's does; it was
// "builtin: not found". busybox has not got it.
func TestBuiltin(t *testing.T) {
	tests := []struct {
		name, script, stdout, stderr string
	}{
		{
			name:   "past a function of the same name",
			script: "echo() { printf 'func:%s\\n' \"$*\"; builtin echo \"real:$*\"; }\necho hi\n",
			stdout: "func:hi\nreal:hi\n",
		},
		{name: "a builtin", script: "builtin cd . && builtin echo ok\n", stdout: "ok\n"},
		{
			name:   "not a builtin",
			script: "builtin nosuch-zz\necho \"st=$?\"\n",
			stdout: "st=1\n", stderr: "builtin: nosuch-zz: not a shell builtin\n",
		},
		{name: "no name", script: "builtin\necho \"st=$?\"\n", stdout: "st=0\n"},
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
