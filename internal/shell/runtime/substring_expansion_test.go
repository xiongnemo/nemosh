package runtime_test

import "testing"

// The offset and length of `${s:offset:length}` are arithmetic, and arithmetic expands
// what is in it first, as $(( )) does. They were evaluated as written, so `${s:$i:2}` --
// the ordinary way to take a piece at a computed place -- stopped the script with a
// syntax error at the `$`, as did `${*:$#}`, the last argument. busybox-w32 and bash 5.3
// agree on each; the array slice is bash's alone.
func TestRuntime_substringOffsetIsExpandedBeforeItIsEvaluated(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`s=abcdef; i=2; echo ${s:$i:2}`, "cd\n"},
		{`s=abcdef; i=2; n=3; echo ${s:i:n}`, "cde\n"},
		{`s=abcdef; i=1; echo "${s:$i}" "${s:$((i+1)):$i}"`, "bcdef c\n"},
		{`s=abcdef; echo ${s:${#s}-2}`, "ef\n"},
		{`s=abcdef; echo ${s:$(echo 3)}`, "def\n"},
		{`set -- a b c; echo ${*:$#}`, "c\n"},
		{`set -- a b c; echo ${@:$(( $# - 1 ))}`, "b c\n"},
		{`a=(p q r); i=1; echo ${a[@]:$i}`, "q r\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0", stdout, status, test.want)
			}
		})
	}
}
