package runtime_test

import "testing"

// Once a function returns, `$LINENO` is the line that called it again, so an ERR trap the
// call's failure fires in the caller names the caller's line, as both references name it. It
// named the function's last line: line=4 three times, where busybox-w32 and bash say 6, 4, 8.
func TestRuntime_linenoIsTheCallersAfterACall(t *testing.T) {
	script := "trap 'echo line=$LINENO' ERR\nfailing() {\n  true\n  false\n}\nfailing\nset -o errtrace\nfailing\n"
	want := "line=6\nline=4\nline=8\n"
	// The script ends with the failing call, so with its status.
	if stdout, status := runScriptCapturing(script); stdout != want || status != 1 {
		t.Errorf("got %q/%d, want %q/1, as both references answer", stdout, status, want)
	}
}
