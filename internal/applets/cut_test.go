package applets

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCutApplet_returnsName_whenConstructed(t *testing.T) {
	// Given
	applet := newCutApplet()

	// When
	got := applet.Name()

	// Then
	if got != "cut" {
		t.Fatalf("expected cut applet name, got %q", got)
	}
}

func TestCutApplet_selectsBytePositions_whenRunWithDashB(t *testing.T) {
	// Given
	applet := newCutApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-b", "1,3"}, bytes.NewBufferString("abcd\nwxyz\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected cut -b to succeed, got %v", err)
	}
	if got, want := stdout.String(), "ac\nwy\n"; got != want {
		t.Fatalf("expected byte selection stdout %q, got %q", want, got)
	}
}

func TestCutApplet_selectsCharacterRange_whenRunWithDashC(t *testing.T) {
	// Given
	applet := newCutApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-c", "2-4"}, bytes.NewBufferString("abcdef\nxy\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected cut -c to succeed, got %v", err)
	}
	if got, want := stdout.String(), "bcd\ny\n"; got != want {
		t.Fatalf("expected character range stdout %q, got %q", want, got)
	}
}

func TestCutApplet_selectsOpenEndedRanges_whenRunWithByteRanges(t *testing.T) {
	// Given
	applet := newCutApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-b", "-2,4-"}, bytes.NewBufferString("abcdef\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected open-ended cut ranges to succeed, got %v", err)
	}
	if got, want := stdout.String(), "abdef\n"; got != want {
		t.Fatalf("expected open-ended range stdout %q, got %q", want, got)
	}
}

func TestCutApplet_ignoresDashN_whenRunWithByteMode(t *testing.T) {
	// Given
	applet := newCutApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-n", "-b", "2"}, bytes.NewBufferString("abc\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected cut -n -b to succeed, got %v", err)
	}
	if got, want := stdout.String(), "b\n"; got != want {
		t.Fatalf("expected ignored -n stdout %q, got %q", want, got)
	}
}

func TestCutApplet_readsStdin_whenRunWithoutFileOperand(t *testing.T) {
	// Given
	applet := newCutApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-c", "1"}, bytes.NewBufferString("abc\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected cut stdin to succeed, got %v", err)
	}
	if got, want := stdout.String(), "a\n"; got != want {
		t.Fatalf("expected stdin stdout %q, got %q", want, got)
	}
}

func TestCutApplet_readsStdin_whenRunWithDashOperand(t *testing.T) {
	// Given
	applet := newCutApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-c", "2", "-"}, bytes.NewBufferString("abc\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected cut - to succeed, got %v", err)
	}
	if got, want := stdout.String(), "b\n"; got != want {
		t.Fatalf("expected dash operand stdout %q, got %q", want, got)
	}
}

func TestCutApplet_readsStdin_whenRunWithOptionTerminator(t *testing.T) {
	// Given
	applet := newCutApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-c", "1", "--"}, bytes.NewBufferString("abc\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected cut -- to succeed, got %v", err)
	}
	if got, want := stdout.String(), "a\n"; got != want {
		t.Fatalf("expected option-terminated stdin stdout %q, got %q", want, got)
	}
}

func TestCutApplet_selectsCharacters_whenRunWithFileOperand(t *testing.T) {
	// Given
	applet := newCutApplet()
	path := writeCutFixture(t, "abcd\n")
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-c", "2-3", path}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected cut file operand to succeed, got %v", err)
	}
	if got, want := stdout.String(), "bc\n"; got != want {
		t.Fatalf("expected file stdout %q, got %q", want, got)
	}
}

func TestCutApplet_selectsCharacters_whenRunWithMultipleFiles(t *testing.T) {
	// Given
	applet := newCutApplet()
	first := writeCutFixture(t, "abcd\n")
	second := writeCutFixture(t, "wxyz\n")
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-c", "2", first, second}, &bytes.Buffer{}, &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected cut multiple files to succeed, got %v", err)
	}
	if got, want := stdout.String(), "b\nx\n"; got != want {
		t.Fatalf("expected multiple file stdout %q, got %q", want, got)
	}
}

func TestCutApplet_normalizesCRLF_whenRunWithWindowsLineEndings(t *testing.T) {
	// Given
	applet := newCutApplet()
	var stdout bytes.Buffer

	// When
	err := applet.Run(context.Background(), []string{"-c", "2-3"}, bytes.NewBufferString("abcd\r\nef\r\n"), &stdout, &bytes.Buffer{})

	// Then
	if err != nil {
		t.Fatalf("expected cut CRLF input to succeed, got %v", err)
	}
	if got, want := stdout.String(), "bc\nf\n"; got != want {
		t.Fatalf("expected CRLF-normalized stdout %q, got %q", want, got)
	}
}

func TestCutApplet_returnsStatusOneAndDiagnostic_whenRunWithInvalidOption(t *testing.T) {
	// Given
	args := []string{"-x"}

	// When
	result := runCutFailure(args)

	// Then
	assertCutFailure(t, result, "cut: invalid option -- x\n")
}

func TestCutApplet_returnsStatusOneAndDiagnostic_whenRunWithUnsupportedOption(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{"unsupported -F", []string{"-F", "1"}, "cut: invalid option -- F\n"},
		{"unsupported -D", []string{"-D", ":", "-f", "1"}, "cut: invalid option -- D\n"},
		{"unsupported -O", []string{"-O:", "-f", "1"}, "cut: invalid option -- O\n"},
		{"unsupported output delimiter", []string{"--output-delimiter=:", "-f", "1"}, "cut: invalid option -- -\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			args := tt.args

			// When
			result := runCutFailure(args)

			// Then
			assertCutFailure(t, result, tt.wantStderr)
		})
	}
}

func TestCutApplet_returnsStatusOneAndDiagnostic_whenRunWithoutMode(t *testing.T) {
	// Given
	args := []string{"--"}

	// When
	result := runCutFailure(args)

	// Then
	assertCutFailure(t, result, "cut: expected a list of bytes, characters, or fields\n")
}

func TestCutApplet_returnsStatusOneAndDiagnostic_whenRunWithMultipleModes(t *testing.T) {
	// Given
	args := []string{"-b", "1", "-c", "1"}

	// When
	result := runCutFailure(args)

	// Then
	assertCutFailure(t, result, "cut: options -b, -c, and -f are mutually exclusive\n")
}

func TestCutApplet_returnsStatusOneAndDiagnostic_whenRunWithInvalidRanges(t *testing.T) {
	tests := []struct {
		name       string
		list       string
		wantStderr string
	}{
		{"empty list", "", "cut: missing list of positions\n"},
		{"dash only", "-", "cut: invalid range -\n"},
		{"zero", "0", "cut: invalid range 0\n"},
		{"leading plus", "+3", "cut: invalid range +3\n"},
		{"leading plus endpoint", "1-+3", "cut: invalid range 1-+3\n"},
		{"negative endpoint", "--3", "cut: invalid range --3\n"},
		{"reversed range", "4-2", "cut: invalid range 4-2\n"},
		{"non-numeric", "a", "cut: invalid range a\n"},
		{"empty segment", "1,,2", "cut: invalid range 1,,2\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			args := []string{"-b", tt.list}

			// When
			result := runCutFailure(args)

			// Then
			assertCutFailure(t, result, tt.wantStderr)
		})
	}
}

func TestCutApplet_returnsStatusOneAndDiagnostic_whenRunWithMissingFile(t *testing.T) {
	// Given
	path := filepath.Join(t.TempDir(), "missing.txt")
	args := []string{"-c", "1", path}

	// When
	result := runCutFailure(args)

	// Then
	assertCutFailure(t, result, "cut: "+path+": No such file or directory\n")
}

func TestCutApplet_returnsStatusOneAndDiagnostic_whenRunWithEmptyFileOperand(t *testing.T) {
	// Given
	args := []string{"-c", "1", ""}

	// When
	result := runCutFailure(args)

	// Then
	assertCutFailure(t, result, "cut: : No such file or directory\n")
}

type cutFailureResult struct {
	err    error
	stdout string
	stderr string
}

func runCutFailure(args []string) cutFailureResult {
	applet := newCutApplet()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := applet.Run(context.Background(), args, bytes.NewBufferString("abc\n"), &stdout, &stderr)
	return cutFailureResult{err: err, stdout: stdout.String(), stderr: stderr.String()}
}

func assertCutFailure(t *testing.T, result cutFailureResult, wantStderr string) {
	t.Helper()
	assertCutStatus(t, result.err, 1)
	if result.stdout != "" {
		t.Fatalf("expected empty stdout, got %q", result.stdout)
	}
	if result.stderr != wantStderr {
		t.Fatalf("expected stderr %q, got %q", wantStderr, result.stderr)
	}
}

func writeCutFixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("expected fixture write to succeed, got %v", err)
	}
	return path
}

func assertCutStatus(t *testing.T, err error, want int) {
	t.Helper()
	got, ok := StatusCode(err)
	if !ok || got != want {
		t.Fatalf("expected applet status %d, got status=%d ok=%v err=%v", want, got, ok, err)
	}
}
