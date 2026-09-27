package runtime_test

import "testing"

// An escaped `$` begins no expansion to the scans that find groups. They stepped over
// `\${...}`, `\$(...)` and `\$((...))` as expansions, with the backslash's escape still
// pending, so the escaped quote after one closed the string: `(a b)` further along was taken
// for a subshell, and its placeholder, __nemosh_group__, landed in the value -- or a group
// holding one was "missing )". busybox-w32 and bash agree on every row.
func TestRuntime_escapedDollarBeginsNoExpansionToTheGroupScans(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`x="\${v-(u)}\" (a b)"; printf '[%s]\n' "$x"`, "[${v-(u)}\" (a b)]\n"},
		{`x="\$(u)\" (a b)"; printf '[%s]\n' "$x"`, "[$(u)\" (a b)]\n"},
		{`x="\$((1))\" (a b)"; printf '[%s]\n' "$x"`, "[$((1))\" (a b)]\n"},
		{`{ echo "\${a}\" (b c)"; }`, "${a}\" (b c)\n"},
		{`( echo "\$(a)\" (b c)" )`, "$(a)\" (b c)\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox-w32 and bash answer", stdout, status, test.want)
			}
		})
	}
}
