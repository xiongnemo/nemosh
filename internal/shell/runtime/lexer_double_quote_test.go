package runtime_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// Inside double quotes a backslash is an escape only before `$`, a backtick, a
// double quote, another backslash, or a newline; before anything else it is an
// ordinary character and has to survive. busybox-w32 ash implements exactly that
// list in `case CBACK` (shell/ash.c:14518) -- it re-emits the backslash when
// `synstack->dblquote` is set and the next character is none of those.
//
// This is what makes the quoted Windows path form in
// docs/design/windows-path-model.md:32 work: `"C:\Users\nemo"` is a path, not a
// string with two escapes in it.
func TestRuntime_keepsABackslashInDoubleQuotesUnlessItEscapesAQuotingCharacter(t *testing.T) {
	tests := []struct {
		name    string
		operand string
		want    string
	}{
		{name: "before an ordinary letter", operand: `"a\Ub"`, want: `a\Ub`},
		{name: "before a digit", operand: `"a\1b"`, want: `a\1b`},
		{name: "before a space", operand: `"a\ b"`, want: `a\ b`},
		{name: "before a slash", operand: `"a\/b"`, want: `a\/b`},
		{name: "windows path", operand: `"C:\Users\nemo\file.txt"`, want: `C:\Users\nemo\file.txt`},
		// The four that stay escapes. (The fifth, a backslash before a newline,
		// never reaches the lexer: line continuation is joined earlier, which
		// TestRuntime_doubleQuotedBackslashNewline_removesContinuation covers.)
		{name: "before a backslash", operand: `"a\\b"`, want: `a\b`},
		{name: "before a dollar", operand: `"a\$b"`, want: `a$b`},
		{name: "before a double quote", operand: `"a\"b"`, want: `a"b`},
		{name: "before a backtick", operand: "\"a\\`b\"", want: "a`b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			var stdout, stderr bytes.Buffer
			rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

			// When
			// printf %s rather than echo so nothing downstream reinterprets a
			// backslash the lexer was supposed to hand over untouched.
			status := rt.RunScript(context.Background(), "printf '%s' "+tt.operand+"\n")

			// Then
			if status != 0 {
				t.Fatalf("expected status 0, got %d: %s", status, stderr.String())
			}
			if got := stdout.String(); got != tt.want {
				t.Errorf("%s expanded to %q, want %q", tt.operand, got, tt.want)
			}
		})
	}
}

// A retained backslash is still inside double quotes, so it is quoted data: it
// must not turn the operand into a glob pattern, and it must not stop `$` from
// expanding on the other side of it.
func TestRuntime_treatsARetainedDoubleQuoteBackslashAsQuotedData(t *testing.T) {
	// Given
	var stdout, stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(),
		"name=nemo\nprintf '[%s]' \"C:\\Users\\x$name\\*.txt\"\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d: %s", status, stderr.String())
	}
	if got, want := stdout.String(), `[C:\Users\xnemo\*.txt]`; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
