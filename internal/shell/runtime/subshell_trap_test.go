package runtime_test

import "testing"

// A subshell runs the EXIT trap it set itself, and not the one it inherited; and the parent's
// traps stay visible to it, so `saved=$(trap)` saves them. Every transcript is busybox-w32's
// and bash's alike, measured.
func TestSubshellExitTrap_runsWhenTheSubshellEnds(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
	}{
		{name: "its own trap runs", script: "(trap 'echo sub-exit' EXIT; echo in-sub)\necho parent\n", want: "in-sub\nsub-exit\nparent\n"},
		{name: "on exit, keeping the status", script: "(trap 'echo sub' EXIT; exit 3)\necho \"st=$?\"\n", want: "sub\nst=3\n"},
		{name: "an inherited trap does not", script: "trap 'echo parent-exit' EXIT\n(echo in-sub)\necho after\n", want: "in-sub\nafter\nparent-exit\n"},
		{name: "in a substitution too", script: "x=$(trap 'echo t' EXIT; echo y)\necho \"[$x]\"\n", want: "[y\nt]\n"},
		{name: "the parent's traps can be saved", script: "trap 'echo p' EXIT\nsaved=$(trap)\necho \"[$saved]\"\n", want: "[trap -- 'echo p' EXIT]\np\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout string
			stdout, _ = runScriptCapturing(test.script)
			if stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}

// `unset -f` removes a function, and a plain `unset` does not reach one -- busybox's reading
// of POSIX. There was no option parsing, so `-f` was taken for a variable's name.
func TestUnset_takesItsOptions(t *testing.T) {
	script := "f() { echo f-ran; }\nunset f\nf\nunset -f f\nf 2>/dev/null || echo gone\nx=1\nunset -v x\necho \"[$x]\"\n"
	if stdout, _ := runScriptCapturing(script); stdout != "f-ran\ngone\n[]\n" {
		t.Fatalf("stdout = %q", stdout)
	}
}
