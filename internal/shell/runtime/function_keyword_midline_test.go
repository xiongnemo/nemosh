package runtime_test

import "testing"

// `function name {` opens the body wherever a command can begin, not only at the start of a
// line, as busybox-w32 and bash both read it: after `;`, `&&`, `then` and `do`. The brace was
// taken for text there, so the `}` lines later closed nothing and the script was "missing }".
func TestRuntime_functionKeywordBraceMidLine(t *testing.T) {
	script := ": prefix; function myfunc {\n\techo serialized\n}\nmyfunc\n" +
		"if true; then function g {\n echo g\n}\nfi; g\n" +
		"true && function h {\n echo h\n}\nh\n" +
		"for i in 1; do function k {\n echo k$i\n}\ndone; k\n" +
		"echo {\n" +
		"x=1; function m { echo m; }; m\n"
	want := "serialized\ng\nh\nk1\n{\nm\n"
	if stdout, status := runScriptCapturing(script); stdout != want || status != 0 {
		t.Errorf("got %q/%d, want %q/0, as busybox-w32 and bash answer", stdout, status, want)
	}
}
