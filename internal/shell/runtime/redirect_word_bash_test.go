package runtime

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// Bash mode expands a redirection's word as bash does outside its POSIX mode, as it expands an
// argument: braces, field splitting and pathname expansion too, a pattern matching nothing
// staying as written. It has to come to one word: none, or more than one, is `WORD: ambiguous
// redirect`, the word as written, status 1, and the command does not run. Under `set -o posix`
// it is one word, as busybox's is, which nemosh and sh keep (redirect_word_test.go). Measured
// 2026-11-22 with Git for Windows' bash 5.3, a script per case.
func TestRuntime_bashModeRedirectionWordIsExpandedAsAnArgument(t *testing.T) {
	tests := []struct {
		script, want, said string
	}{
		{script: "x='sp ace'; echo one > $x; echo st=$?; ls", want: "st=1\n", said: "line 2: $x: ambiguous redirect\n"},
		{script: "echo two > a-{p,q}; echo st=$?; ls", want: "st=1\n", said: "a-{p,q}: ambiguous redirect\n"},
		{script: "e=; echo three > $e; echo st=$?; ls", want: "st=1\n", said: "$e: ambiguous redirect\n"},
		{script: "set -- a b; echo four > \"$@\"; echo st=$?; ls", want: "st=1\n", said: "\"$@\": ambiguous redirect\n"},
		{script: ": > foo-bar; : > foo-spam; echo five > foo-*; echo st=$?; cat foo-bar foo-spam", want: "st=1\n", said: "foo-*: ambiguous redirect\n"},
		{script: ": > one-bar; echo six > one-*; cat one-bar", want: "six\n"},
		{script: "star='*'; : > two-bar; echo seven > two-$star; cat two-bar", want: "seven\n"},
		{script: "echo eight > zz-{q}; cat 'zz-{q}'", want: "eight\n"},
		{script: "x='sp ace'; echo nine > \"$x\"; cat \"$x\"", want: "nine\n"},
		{script: "shopt -s nullglob; echo ten > none-*; echo st=$?", want: "st=1\n", said: "none-*: ambiguous redirect\n"},
		{script: "shopt -s failglob; echo x; echo eleven > none-*; echo same-line\necho st=$?", want: "x\nst=1\n", said: "line 2: no match: none-*\n"},
		{script: "set -o posix; x='sp ace'; echo twelve > $x; cat 'sp ace'", want: "twelve\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
			rt.EnterBashMode()
			rt.RunScript(context.Background(), fmt.Sprintf("cd '%s'\n%s\n", filepath.ToSlash(t.TempDir()), test.script))
			said := stderr.String()
			if stdout.String() != test.want || test.said != "" && !strings.HasSuffix(said, test.said) || test.said == "" && said != "" {
				t.Errorf("stdout %q, stderr %q; want %q and %q", stdout.String(), said, test.want, test.said)
			}
		})
	}
}
