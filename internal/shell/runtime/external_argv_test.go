package runtime_test

import (
	"strings"
	"testing"
)

// Nemosh must not rewrite ordinary argv on the way to a native program, which
// is the MSYS2 behaviour docs/design/windows-path-model.md:129 explicitly
// declines: a path-shaped operand is often not a path, and converting it
// corrupts regexes, URLs, and refspecs. The policy is stated as an absence, so
// nothing failed when it was only an absence -- this pins it as a promise.
//
// Every operand here is one Nemosh could resolve if it wanted to: two virtual
// roots, a drive alias, a mount alias, a UNC share, plus three strings that
// merely look like paths. All of them have to arrive byte for byte.
func TestRuntime_passesPathShapedArgumentsToAChildUnconverted(t *testing.T) {
	// Given
	operands := []string{
		"/c/Users/nemo/file.txt",
		"/mnt/c/tmp",
		"//host/share/dir",
		"/tmp/report.txt",
		"/dev/null",
		"https://example.com/a/b",
		"refs/heads/main",
	}
	// Quoted because of the pipes, and included because a sed script is the
	// classic thing argv conversion breaks.
	script := "s|/usr/bin/env|/opt/bin/env|"

	// When
	status, stdout, stderr := runHelperWith(t, nil,
		"argv "+strings.Join(operands, " ")+" '"+script+"'")

	// Then
	if status != 0 {
		t.Fatalf("expected the child to run, status=%d stderr=%q", status, stderr)
	}
	want := strings.Join(append(operands, script), "\n") + "\n"
	if stdout != want {
		t.Errorf("child saw:\n%q\nwant:\n%q", stdout, want)
	}
}
