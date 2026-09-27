package runtime_test

import "testing"

// A heredoc's body begins after the line its command is on has ended, and a line left inside
// a quote or ending in a backslash goes on into the next, as busybox-w32 and bash both read
// it. The body was taken from the very next line, so each of these was refused as incomplete
// or ran the wrong thing, and a quote one line left open hid the next line's `<<`.
func TestRuntime_heredocBodyBeginsAfterTheLogicalLine(t *testing.T) {
	for index, test := range []struct {
		script, want string
		status       int
	}{
		{"cat <<EOF \\\n; echo two\none\nEOF\n", "one\ntwo\n", 0},
		{"cat <<EOF; echo \"two\nthree\"\none\nEOF\n", "one\ntwo\nthree\n", 0},
		{"cat <<EOF; echo 'a\nb'\nbody\nEOF\n", "body\na\nb\n", 0},
		{"cat <<EOF; echo $'a\nb'\nx\nEOF\n", "x\na\nb\n", 0},
		{"cat <<EOF; echo \"a\\\nb\"\nx\nEOF\n", "x\nab\n", 0},
		{"cat <<EOF; echo 'a\\\nb'\nx\nEOF\n", "x\na\\\nb\n", 0},
		{"cat <<A \\\n&& cat <<B\none\nA\ntwo\nB\n", "one\ntwo\n", 0},
		{"echo \"a\nb\" <<EOF\nx\nEOF\necho after\n", "a\nb\nafter\n", 0},
		// The continued line is the command's: `1` is cat's operand, the body is `2`, and a
		// `|` on a line of its own is a syntax error.
		{"cat <<EOF \\\n1\n2\nEOF\n| cat\necho notreached\n", "", 2},
		// A comment and a body hold quotes that open nothing.
		{"# don't\ncat <<EOF\nx\nEOF\n", "x\n", 0},
		{"cat <<A\nit's\nA\ncat <<B\nb\nB\n", "it's\nb\n", 0},
	} {
		if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != test.status {
			t.Errorf("%d: %q: got %q/%d, want %q/%d, as busybox-w32 and bash answer", index, test.script, stdout, status, test.want, test.status)
		}
	}
}
