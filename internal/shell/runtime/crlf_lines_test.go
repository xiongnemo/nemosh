package runtime_test

import (
	goruntime "runtime"
	"testing"
)

// On Windows a CRLF a program writes is a newline, as busybox-w32 reads it: a command
// substitution's trailing newlines go with the CR of each CRLF among them -- bash does that
// there too -- and a field ends before a CRLF's CR; a CR anywhere else is a character. Each
// kept its CR, so `set -- $(where git)` had one on every word. Each answer is busybox-w32's.
func TestCRLF_aWindowsProgramsLinesAreLines(t *testing.T) {
	if goruntime.GOOS != "windows" {
		t.Skip("a CR is a character elsewhere, as busybox and bash have it there")
	}
	script := `x=$(printf 'a b\r\nc\r\n\r\n'); printf '%s|' "$x"
set -- $x; printf '[%s]' "$@"
y="p q$(printf '\r')"; set -- $y; printf '[%s]' "$@"
z=$(printf 'a\rb\r'); printf '%s|' "$z"
`
	want := "a b\r\nc|[a][b][c][p][q\r]a\rb\r|"
	if status, stdout, stderr := runSetScript(t, script); status != 0 || stdout != want || stderr != "" {
		t.Errorf("got %d/%q/%q, want 0 and %q", status, stdout, stderr, want)
	}
}
