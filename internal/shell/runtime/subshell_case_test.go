package runtime_test

import "testing"

// A case in a subshell may run over several lines: its patterns' `)` are theirs, and the
// subshell's is the last. The first pattern's closed the subshell, so each script was refused.
// Both references print every line, measured.
func TestSubshell_aCaseInsideSpansLines(t *testing.T) {
	script := "(case a in\n\ta) echo y;;\nesac\n)\n" +
		"(for d in a\ndo\n case $d in\n a) echo z;;\n esac\ndone\n)\n" +
		"f() {\n\t(\n\t\tcase b in\n\t\t(b) echo w;;\n\t\tesac\n\t)\n}\nf\n"
	want := "y\nz\nw\n"
	if stdout, status := runScriptCapturing(script); stdout != want || status != 0 {
		t.Fatalf("got %q/%d, want %q/0, as both references answer", stdout, status, want)
	}
}
