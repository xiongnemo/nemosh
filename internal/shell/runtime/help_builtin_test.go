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

// Operands are refused rather than ignored: busybox's help takes none either,
// and silently discarding one would hide a typo.
func TestHelp_refusesOperands(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "help cd\n")

	// Then
	if status == 0 {
		t.Fatalf("help cd succeeded, want a refusal; stdout = %q", stdout)
	}
	if !strings.Contains(stderr, "help") {
		t.Fatalf("stderr = %q, want it to name help", stderr)
	}
}
