package runtime_test

import "testing"

// `$@` and `$*`, quoted and not, braced and not -- every answer busybox-w32's and bash's
// alike, measured. The unquoted forms kept each parameter whole and kept the empty ones;
// `"$*"` joined with a space whatever IFS said; `"${@}"` was one word.
func TestPositionalParameters_splitAndJoinAsPOSIXSays(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
	}{
		{name: "unquoted $@ splits and drops empties", script: "set -- 'a b' '' c\nfor x in $@; do printf '[%s]' \"$x\"; done; echo\n", want: "[a][b][c]\n"},
		{name: "unquoted $* the same", script: "set -- 'a b' '' c\nfor x in $*; do printf '[%s]' \"$x\"; done; echo\n", want: "[a][b][c]\n"},
		{name: "quoted $@ keeps everything", script: "set -- 'a b' '' c\nfor x in \"$@\"; do printf '[%s]' \"$x\"; done; echo\n", want: "[a b][][c]\n"},
		{name: "an empty IFS still separates parameters", script: "set -- 'a b' '' c\nIFS=\nfor x in $@; do printf '[%s]' \"$x\"; done; echo\n", want: "[a b][c]\n"},
		{name: "a prefix and a suffix join the ends", script: "set -- 'a b' '' c\nfor x in x$@y; do printf '[%s]' \"$x\"; done; echo\n", want: "[xa][b][cy]\n"},
		{name: "\"$*\" joins by IFS", script: "set -- a b\nIFS=,\necho \"$*\"\n", want: "a,b\n"},
		{name: "\"$*\" with an empty IFS", script: "set -- a b\nIFS=\necho \"$*\"\n", want: "ab\n"},
		{name: "the braced forms", script: "set -- 'a b' c\nfor x in \"${@}\"; do printf '[%s]' \"$x\"; done\nIFS=,\necho \" ${*}\"\n", want: "[a b][c] a b,c\n"},
		{name: "in an assignment", script: "set -- 'a b' '' c\nx=$@\nIFS=,\ny=$*\necho \"[$x][$y]\"\n", want: "[a b  c][a b,,c]\n"},
		{name: "an array's star form", script: "a=(x y)\nIFS=-\necho \"${a[*]}\"\n", want: "x-y\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if stdout, _ := runScriptCapturing(test.script); stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}
