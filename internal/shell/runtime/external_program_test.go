package runtime_test

import (
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"
)

// writeTestProgram puts a program that says its arguments and $NEMOSH_TEST_PROGRAM_VAR in dir,
// named nemosh-test-program: a batch file on Windows, which ComSpec runs, and a script elsewhere.
// Not a shebang script on Windows, where this shell runs one, and the shell's binary in a test is
// the test binary.
func writeTestProgram(t *testing.T, dir string) {
	t.Helper()
	name, text := "nemosh-test-program", "#!/bin/sh\necho \"program $* [$NEMOSH_TEST_PROGRAM_VAR]\"\n"
	if goruntime.GOOS == "windows" {
		name, text = "nemosh-test-program.bat", "@echo program %* [%NEMOSH_TEST_PROGRAM_VAR%]\r\n"
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o755); err != nil {
		t.Fatalf("write the program: %v", err)
	}
}

// env, xargs, find's -exec and awk's system() run a program no applet has, found on PATH as a
// command is, where each was `not found` whatever PATH held. env passes on what it sets, and
// looks the program up on a PATH it sets.
func TestApplets_runAProgramNoAppletHas(t *testing.T) {
	dir, elsewhere := t.TempDir(), t.TempDir()
	writeTestProgram(t, dir)
	writeTestProgram(t, elsewhere)
	if err := os.WriteFile(filepath.Join(dir, "found.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("NEMOSH_TEST_PROGRAM_VAR", "")
	shellDir := filepath.ToSlash(dir)

	for _, test := range []struct {
		script string
		want   string
	}{
		{script: "echo a b | xargs nemosh-test-program", want: "program a b []"},
		{script: "find '" + shellDir + "' -name found.txt -exec nemosh-test-program here \\;", want: "program here []"},
		{script: "env NEMOSH_TEST_PROGRAM_VAR=set nemosh-test-program x", want: "program x [set]"},
		{script: "awk 'BEGIN { system(\"nemosh-test-program q\") }'", want: "program q []"},
		{script: "env PATH='" + filepath.ToSlash(elsewhere) + "' nemosh-test-program y", want: "program y []"},
	} {
		t.Run(test.script, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script+"\n")

			// Then
			got := strings.TrimSpace(strings.ReplaceAll(stdout, "\r\n", "\n"))
			if status != 0 || got != test.want {
				t.Fatalf("status %d, stdout %q, stderr %q; want 0 and %q", status, stdout, stderr, test.want)
			}
		})
	}
}

// A name no applet has and PATH does not either is still not found, in each applet's words.
func TestApplets_sayAProgramIsNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	status, _, stderr := runSetScript(t, "echo q | xargs nemosh-no-such-program\n")
	if status != 127 || !strings.Contains(stderr, "nemosh-no-such-program") {
		t.Fatalf("status %d, stderr %q; want 127 naming the program", status, stderr)
	}
}
