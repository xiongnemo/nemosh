package runtime_test

import "testing"

// An alias is substituted from the aliases a command's line was read with, as busybox and bash
// substitute it: not one defined earlier on the same line or in the same compound, and inside a
// function what was defined when the function was, not when it is called. eval's text, a
// sourced file and a job are read the same way. Each came out the other way, the alias in
// force as the command ran. Both references agree on every case; bash with expand_aliases.
//
// Each name is one no system runs as a command, since a name not yet an alias runs whatever
// PATH holds: `w` is who is logged in on macOS and Linux, and it printed there.
func TestRuntime_aliasIsInForceFromTheNextLine(t *testing.T) {
	for _, test := range []struct{ script, want string }{
		{"alias e=echo; e one\ne two\n", "two\n"},
		{"alias e=echo\nf() { e hi; }\nunalias e\nf\n", "hi\n"},
		{"g() { e hi; }\nalias e=echo\ng\necho done\n", "done\n"},
		{"if true; then alias q=echo; q inside; fi\nq after\n", "after\n"},
		{"eval 'alias wz=echo; wz same'\nwz next\n", "next\n"},
		{"eval 'alias v=echo\nv second'\n", "second\n"},
		{"h() { alias z=echo; }\nh; z same\nz next\n", "next\n"},
		{"alias ll='echo ll:'\nk() { ll inside; }\nk\n", "ll: inside\n"},
		{"alias e=echo; e same & wait\ne next & wait\n", "next\n"},
		{"alias e=echo; (e same)\n(e next)\n", "next\n"},
		{"alias e=echo; x=$(e same); echo \"[$x]\"\n", "[]\n"},
		{"alias e=echo; { e same; } | cat\ne next | cat\n", "next\n"},
		{"(\nalias fz=echo\nfz same\n)\necho after\n", "after\n"},
		{"alias e=echo; type e\n", "e is an alias for echo\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			if stdout, _ := runScriptCapturing(test.script); stdout != test.want {
				t.Errorf("got %q, want %q, as busybox and bash answer", stdout, test.want)
			}
		})
	}
}
