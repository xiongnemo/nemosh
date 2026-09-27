package runtime_test

import "testing"

// `shopt -u expand_aliases` stops alias expansion, and `shopt -s expand_aliases` starts it
// again. A new shell has it on, since busybox expands aliases in a script; a bash script
// starts with it off, and turns it on first. ble.sh turns it off around an eval so that its
// own aliases cannot change the code it runs.
func TestShopt_expandAliases(t *testing.T) {
	tests := []struct {
		name, script, stdout string
		status               int
	}{
		{name: "on in a new shell", script: "alias e=echo\nshopt -q expand_aliases && e on\n", stdout: "on\n"},
		{
			name:   "off, the name is a command name",
			script: "alias e=echo\nshopt -u expand_aliases\ne hi 2>/dev/null\necho \"st=$?\"\n",
			stdout: "st=127\n",
		},
		{name: "on again", script: "alias e=echo\nshopt -u expand_aliases\nshopt -s expand_aliases\ne hi\n", stdout: "hi\n"},
		{
			name:   "around an eval",
			script: "alias echo=false\nf() {\n  shopt -u expand_aliases\n  eval \"$1\"\n  shopt -s expand_aliases\n}\nf 'echo hello'\n",
			stdout: "hello\n",
		},
		{
			// An alias of a declaration utility is not one either.
			name:   "a declaration alias",
			script: "alias e=export\nshopt -u expand_aliases\nw='a b'\ne x=$w 2>/dev/null\necho \"st=$? [${x-unset}]\"\n",
			stdout: "st=127 [unset]\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if stdout != test.stdout || status != test.status {
				t.Fatalf("got %q/%d, want %q/%d; stderr = %q", stdout, status, test.stdout, test.status, stderr)
			}
		})
	}
}
