package runtime_test

import "testing"

// An unquoted heredoc expands `$((...))` and backquotes, as POSIX says and busybox-w32
// does; `$((1+1))` ran a command named 1+1 and backquotes came out as written. A nested
// `$((...))` anywhere was a command too. Each answer is busybox's, measured.
func TestHeredoc_expandsArithmeticAndBackquotes(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
	}{
		{name: "arithmetic", script: "x=4\ncat <<END\n$(( 2 * 3 )) $((x*2)) $(( (1+2)*3 ))\nEND\n", want: "6 8 9\n"},
		{name: "backquotes, and escaped ones", script: "cat <<END\n`echo bq` \\`lit\\` \\$x\nEND\n", want: "bq `lit` $x\n"},
		{name: "a quoted delimiter expands nothing", script: "cat <<'END'\n`echo q` $((1+1))\nEND\n", want: "`echo q` $((1+1))\n"},
		{name: "nested arithmetic", script: "i=2\necho $((1+$((2)))) $(( i + $((i*3)) ))\n", want: "3 8\n"},
		{name: "arithmetic in an operator's word", script: "x=\necho ${x:-$((1+1))}\n", want: "2\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if stdout, _ := runScriptCapturing(test.script); stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}
