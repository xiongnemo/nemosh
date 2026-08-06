package runtime_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// The lexer walks bytes, which is right for a POSIX shell -- the operators and
// quoting characters are all ASCII and a UTF-8 lead byte can never be mistaken
// for one. What it must not do is *interpret* a byte it does not care about.
// A word carries its bytes through unchanged, whatever encoding they spell.
func TestRuntime_carriesNonASCIIWordsThrough_whateverTheQuoting(t *testing.T) {
	cases := []struct {
		name   string
		script string
		stdout string
	}{
		{name: "unquoted", script: "echo 你好世界\n", stdout: "你好世界\n"},
		{name: "single quoted", script: "echo '你好世界'\n", stdout: "你好世界\n"},
		{name: "double quoted", script: "echo \"你好世界\"\n", stdout: "你好世界\n"},
		{name: "assigned and expanded", script: "greeting=你好\necho $greeting\n", stdout: "你好\n"},
		{name: "backslash escaped", script: "echo \\你\n", stdout: "你\n"},
		{name: "mixed with ascii", script: "echo a你b好c\n", stdout: "a你b好c\n"},
		{name: "emoji", script: "echo 🌏\n", stdout: "🌏\n"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// Given
			var stdout, stderr bytes.Buffer
			rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

			// When
			status := rt.RunScript(context.Background(), testCase.script)

			// Then
			if status != 0 {
				t.Fatalf("expected status 0, got %d with stderr %q", status, stderr.String())
			}
			if got := stdout.String(); got != testCase.stdout {
				t.Fatalf("expected stdout %q, got %q", testCase.stdout, got)
			}
		})
	}
}
