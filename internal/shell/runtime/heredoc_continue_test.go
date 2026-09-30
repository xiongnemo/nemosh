package runtime_test

import "testing"

// A backslash continues a heredoc's operator, its word, and the line that ends an unquoted
// heredoc's body onto the next line, as busybox-w32 and bash 5.3 both read them; each answer
// below was measured in both. The first five were refused as malformed redirections, and the
// body of the sixth never ended.
func TestHeredoc_aBackslashContinuesItsOperatorWordAndDelimiter(t *testing.T) {
	for _, test := range []struct{ name, script, want string }{
		{name: "the operator", script: "cat <\\\n<EOF\nbody\nEOF\n", want: "body\n"},
		{name: "after the operator", script: "cat <<\\\nEOF\nbody\nEOF\n", want: "body\n"},
		{name: "a blank before the word", script: "cat <<\\\n EOF\nbody\nEOF\n", want: "body\n"},
		{name: "the dash", script: "cat <<\\\n-EOF\n\tbody\n\tEOF\n", want: "body\n"},
		{name: "the word", script: "cat << EO\\\nF\nbody\nEOF\n", want: "body\n"},
		{name: "the delimiter's line", script: "cat <<EOF\nbody\nEO\\\nF\necho after\n", want: "body\nafter\n"},
		{name: "the delimiter's line, tabs stripped", script: "cat <<-EOF\n\tbody\n\tEO\\\nF\n", want: "body\n"},
		{name: "a line goes on inside itself", script: "cat <<-EOF\n\ta\\\n\tb\n\tEOF\n", want: "a\tb\n"},
		{name: "a line that goes on is no delimiter", script: "cat <<EOF\na\\\nEOF\nEOF\n", want: "aEOF\n"},
		{name: "a quoted word's body keeps its backslashes", script: "cat <<'EOF'\nEO\\\nF\nEOF\n", want: "EO\\\nF\n"},
		{name: "the lines after count from the first", script: "cat <<EO\\\nF\nx\nEOF\necho $LINENO\n", want: "x\n5\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			status, stdout, stderr := runSetScript(t, test.script)
			if status != 0 || stdout != test.want {
				t.Fatalf("status = %d, stdout = %q, stderr = %q, want 0 and %q", status, stdout, stderr, test.want)
			}
		})
	}
}
