package runtime_test

import (
	"fmt"
	"path/filepath"
	"testing"
)

// `.` and source take `--` before the file, as busybox's `.` does and bash's source; it was
// read as the file's name.
func TestRuntime_dotTakesDashDash(t *testing.T) {
	script := fmt.Sprintf("cd '%s'\n", filepath.ToSlash(t.TempDir())) +
		"echo 'echo foo' > f.sh\n. -- ./f.sh\nsource -- ./f.sh\n"
	if stdout, status := runScriptCapturing(script); stdout != "foo\nfoo\n" || status != 0 {
		t.Errorf("got %q/%d, want %q/0, as both references answer", stdout, status, "foo\nfoo\n")
	}
}

// command -v reports a reserved word by name and an alias as the definition that makes it, as
// busybox reports them: `command -v for` said nothing and exited 1, and an alias was not found.
// bash's keywords that this shell has -- `[[`, `]]`, `function`, `coproc` -- are reserved
// words to `type` and `command -v` too, as bash calls them; `type [[` and `command -v [[` named
// whatever `[[.exe` there was on PATH.
func TestRuntime_commandVReportsKeywordsAndAliases(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{"command -v for; echo st=$?", "for\nst=0\n"},
		{"command -v '[['; command -v '!'; command -v '{'; command -v function", "[[\n!\n{\nfunction\n"},
		{"alias ll='ls -l'; command -v ll; echo st=$?", "alias ll='ls -l'\nst=0\n"},
		{"type '[['; type ']]'; type function; type coproc", "[[ is a shell keyword\n]] is a shell keyword\nfunction is a shell keyword\ncoproc is a shell keyword\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as the references answer", stdout, status, test.want)
			}
		})
	}
}
