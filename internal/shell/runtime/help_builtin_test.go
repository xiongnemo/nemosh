package runtime_test

import (
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// `help` is a builtin in busybox ash, which is why a user there never reaches
// Windows' own help.exe. Without it, `help` inside Nemosh runs help.exe, whose
// output is in the console code page -- CP936 on a Chinese Windows -- and
// arrives as mojibake in a UTF-8 terminal. Measured: busybox produces the same
// bytes when forced to run `help.exe`, so the difference was never encoding
// handling. It was this builtin.
func TestHelp_listsBuiltinsAndApplets(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "help\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q, want 0", status, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	for _, want := range []string{"Built-in commands", "Applets"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("help output does not have a %q section:\n%s", want, stdout)
		}
	}
	// Every builtin the shell dispatches has to appear, or the list becomes a
	// second source of truth that drifts.
	for _, want := range []string{
		".", ":", "alias", "break", "cd", "command", "continue", "eval", "exec",
		"exit", "export", "let", "local", "read", "readonly", "return", "set",
		"shift", "times", "trap", "type", "unalias", "unset", "wait", "jobs",
		"umask", "getopts", "shift", "help",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("help does not list the builtin %q", want)
		}
	}
	// And every applet, read from the registry rather than typed in.
	for _, name := range applets.DefaultRegistry.Names() {
		if !strings.Contains(stdout, name) {
			t.Errorf("help does not list the applet %q", name)
		}
	}
}

// The builtin has to win over the external help.exe, which is the whole point.
func TestHelp_isABuiltin_notAnExternalCommand(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "command -v help\ntype help\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q, want 0", status, stderr)
	}
	if !strings.Contains(stdout, "help") {
		t.Fatalf("command -v help said %q", stdout)
	}
	if !strings.Contains(stdout, "builtin") {
		t.Fatalf("type help said %q, want it to name a builtin", stdout)
	}
	// A resolved path would mean help.exe won.
	for _, wrong := range []string{".exe", "\\", "System32"} {
		if strings.Contains(stdout, wrong) {
			t.Fatalf("help resolved to an external program: %q", stdout)
		}
	}
}

// `help NAME` describes a builtin, as bash's does, and an applet as its --help does; and
// `NAME --help` says the same of a builtin. busybox's help takes no operand, and this one
// refused them, so `help cd` -- the first thing anyone types -- was an error. A name there is
// no help for is still one, so a typo is not hidden.
func TestHelp_describesTheNamesItIsGiven(t *testing.T) {
	for _, test := range []struct {
		script string
		want   []string
	}{
		{"help cd\n", []string{"Usage: cd [-L|-P] [DIRECTORY|-]\n", "Change the working directory."}},
		{"help cd jobs\n", []string{"Usage: cd ", "Usage: jobs "}},
		{"help ls\n", []string{"Usage: ls "}},
		{"help history\n", []string{"Usage: history [-acnprsw] [-d OFFSET] [N]\n"}},
		{"cd --help\n", []string{"Usage: cd ", "Change the working directory."}},
		{"jobs --help | head -1\n", []string{"Usage: jobs "}},
		{"help complete\n", []string{"complete is a shell builtin", "support-matrix"}},
	} {
		status, stdout, stderr := runSetScript(t, test.script)
		for _, want := range test.want {
			if status != 0 || !strings.Contains(stdout, want) {
				t.Errorf("%q = %d, %q, %q; want 0 and %q", test.script, status, stdout, stderr, want)
			}
		}
	}
	status, stdout, stderr := runSetScript(t, "help nosuch\n")
	if status != 1 || stdout != "" || !strings.Contains(stderr, "help: no help topics match `nosuch'") {
		t.Fatalf("help nosuch = %d, %q, %q", status, stdout, stderr)
	}
}
