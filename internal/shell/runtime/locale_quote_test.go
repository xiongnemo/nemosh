package runtime_test

import "testing"

// `$"..."` is bash's string for translating, which with no message catalog is the
// double-quoted string itself: the `$` goes, in a word and in an operator's word, in double
// quotes or not. It stayed, so `echo $"hello"` printed `$hello`, as busybox-w32 prints it,
// which has no such string; bash decides, as the user chose. A heredoc keeps it, as bash's
// does, and a `$` inside double quotes is still a `$`.
func TestRuntime_localeStringIsItsDoubleQuotedString(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`x=1; echo $"x is $x" a$"b"c`, "x is 1 abc\n"},
		{`echo "in $"quotes"" \$"esc" '$"single"'`, "in $quotes $esc $\"single\"\n"},
		{`a=($"one two" three); echo "${#a[@]} ${a[0]}"`, "2 one two\n"},
		{`[[ $"z" == z ]] && echo cond; case $"k" in k) echo case;; esac`, "cond\ncase\n"},
		{`echo ${u:-$"def"} "${u:-$"def"}"`, "def def\n"},
		{`x=abc; echo ${x/$"b"/$"X"} "${x%$"c"}" ${x#$"a"}`, "aXc ab bc\n"},
		{"cat <<EOT\n$\"kept\"\nEOT", "$\"kept\"\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}
