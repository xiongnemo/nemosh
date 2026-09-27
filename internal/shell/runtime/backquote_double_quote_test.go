package runtime_test

import "testing"

// Inside double quotes a backquoted command's `\"` is a quote, its backslash dropped as the
// ones before $, ` and \ are: "x `echo \"hi\"`" runs echo "hi" and is x hi, in busybox-w32 and
// bash alike. The backslash was kept, so echo printed the quotes. Unquoted it stays, and the
// inner echo is given \".
func TestRuntime_backquoteInDoubleQuotesTakesAnEscapedQuote(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{"echo \"x `echo \\\"hi\\\"`\"", "x hi\n"},
		{"echo 1 `echo \\\"`", "1 \"\n"},
		{"echo \"x `echo \"hi\"`\" \"x $(echo \\\"hi\\\")\"", "x hi x \"hi\"\n"},
		// A heredoc's backquote is read as in double quotes, busybox's answer; bash keeps the
		// backslash there and prints "hi".
		{"cat <<EOF\n`echo \\\"hi\\\"`\nEOF", "hi\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox-w32 and bash answer", stdout, status, test.want)
			}
		})
	}
}
