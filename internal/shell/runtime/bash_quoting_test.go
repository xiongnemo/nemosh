package runtime_test

import "testing"

// `printf %q`, `${x@Q}`, `${x@A}` and `declare -p` write a value so that the shell reads it
// back as itself, and each writes it as bash 5.3 does. A value with a newline or another
// character no other quoting holds is $'...' in all of them. declare -p double-quotes the
// rest with $ and ` escaped, and quotes an associative key only when the key needs it.
// declare -p used Go's quoting, so `x=$'a\nb'` came back as the two characters \n, and
// `s='$HOME'` came back expanded. busybox has none of these forms.
func TestRuntime_quotingIsBashs(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`printf '%q\n' $'one\ntwo' 'a=b' 'x#' '~x' $'\x01'`, "$'one\\ntwo'\na=b\nx#\n\\~x\n$'\\001'\n"},
		{`z=$'one\ntwo'; echo "${z@Q}"; echo "${z@A}"; declare -p z`, "$'one\\ntwo'\nz=$'one\\ntwo'\ndeclare -- z=$'one\\ntwo'\n"},
		{"s=\"it's \\$x \\\"q\\\" \\\\ \\`c\\`\"; declare -p s; echo \"${s@Q}\"", "declare -- s=\"it's \\$x \\\"q\\\" \\\\ \\`c\\`\"\n'it'\\''s $x \"q\" \\ `c`'\n"},
		{`q="'"; echo "${q@Q}"`, "\\'\n"},
		{`a=(x "y z" $'p\tq'); declare -p a`, "declare -a a=([0]=\"x\" [1]=\"y z\" [2]=$'p\\tq')\n"},
		{`declare -A m=(["k 1"]=v); m[plain]=w; declare -p m`, "declare -A m=([\"k 1\"]=\"v\" [plain]=\"w\" )\n"},
		{`x=$'a\nb'; s='$HOME'; eval "$(declare -p x s)"; [ "$x" = $'a\nb' ] && [ "$s" = '$HOME' ] && echo round-trip`, "round-trip\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}
