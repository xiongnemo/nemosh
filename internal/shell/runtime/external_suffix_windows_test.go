//go:build windows

package runtime

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// The lookup rule comes from busybox-w32 add_win32_extension (win32/mingw.c:2237):
// the bare name is tried first and accepted when it carries an executable suffix
// or sniffs executable, and only a name with neither gets the five suffixes
// appended in order.
func TestExecutableCandidate_windowsTriesTheBareNameBeforeAppendingSuffixes(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"shebang":         "#!/bin/sh\necho hi\n",
		"plain":           "just words\n",
		"notes.txt":       "just words\n",
		"wrapped.txt":     "just words\n",
		"wrapped.txt.exe": "MZ\x90\x00stand-in for a program\n",
		"tool":            "just words\n",
		"tool.sh":         "echo hi\n",
		"real.exe":        "MZ\x90\x00stand-in for a program\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	// A file that is there and is no program is refused as one, busybox-w32's 126, where a name
	// that is not there at all is not found.
	for _, testCase := range []struct {
		candidate string
		want      string
		refusal   error
	}{
		{candidate: "shebang", want: "shebang"},
		{candidate: "real.exe", want: "real.exe"},
		{candidate: "wrapped.txt", want: "wrapped.txt.exe"},
		{candidate: "tool", want: "tool.sh"},
		{candidate: "plain", refusal: errExternalNotExecutable},
		{candidate: "notes.txt", refusal: errExternalNotExecutable},
		{candidate: "missing", refusal: errExternalNotFound},
	} {
		t.Run(testCase.candidate, func(t *testing.T) {
			got, err := executableCandidate(filepath.Join(dir, testCase.candidate))

			if testCase.want == "" {
				if !errors.Is(err, testCase.refusal) {
					t.Fatalf("executableCandidate(%q) = %q, %v, want %v", testCase.candidate, got, err, testCase.refusal)
				}
				return
			}
			if err != nil {
				t.Fatalf("executableCandidate(%q): %v", testCase.candidate, err)
			}
			if want := filepath.Join(dir, testCase.want); got != want {
				t.Fatalf("executableCandidate(%q) = %q, want %q", testCase.candidate, got, want)
			}
		})
	}
}
