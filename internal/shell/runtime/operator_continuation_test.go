package runtime_test

import "testing"

// A line ending in `&&`, `||` or `|` goes on past blank lines and comments to the command after
// it, and so does one whose quote or substitution began on an earlier line. nemosh read each as
// a command missing its second half. Both references run every line, measured.
func TestContinuation_anOperatorWaitsPastBlankAndCommentLines(t *testing.T) {
	script := "true &&\n# c\necho y1\ntrue ||\n\necho n\necho y2\n" +
		"f() {\n\ttrue &&\n\t# c\n\techo y3\n}\nf\n" +
		"x=\"$(case a in\na) echo z;;\nesac 2>/dev/null)\" ||\necho n\necho \"$x\"\n" +
		"echo a |\n\n# c\ncat\necho 'b\nc' |\ncat\n"
	want := "y1\ny2\ny3\nz\na\nb\nc\n"
	if stdout, status := runScriptCapturing(script); stdout != want || status != 0 {
		t.Fatalf("got %q/%d, want %q/0, as both references answer", stdout, status, want)
	}
}
