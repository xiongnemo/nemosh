package runtime_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// externalEchoOnPath puts a program named `echo` -- this test binary, which
// answers as the helper process -- on a PATH of its own, so that the one name
// exists both as a bundled applet and as an external program. Nothing else the
// registry offers has a counterpart there, which is what lets a single override
// value be observed from both sides.
func externalEchoOnPath(t *testing.T) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("expected test executable path, got %v", err)
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatalf("expected test executable read to succeed, got %v", err)
	}
	name := "echo"
	if goruntime.GOOS == "windows" {
		name += ".exe"
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o700); err != nil {
		t.Fatalf("expected external echo write to succeed, got %v", err)
	}
	t.Setenv("NEMOSH_RUNTIME_HELPER_PROCESS", "1")
	t.Setenv("PATH", dir)
}

// The helper prints the word after `--` and nothing else, so its output is
// distinguishable from the echo applet's, which would repeat the whole line.
const (
	overrideHelperArgs   = "-test.run=TestRuntimeHelperProcess -- external-echo\n"
	overrideHelperStdout = "external-echo\n"
	overrideAppletStdout = "-test.run=TestRuntimeHelperProcess -- external-echo\n"
)

func TestRuntime_prefersTheExternal_whenTheOverrideNamesTheApplet(t *testing.T) {
	cases := []struct {
		name     string
		override string
		stdout   string
	}{
		{name: "no override keeps the applet", override: "", stdout: overrideAppletStdout},
		{name: "an unrelated name keeps the applet", override: "cat", stdout: overrideAppletStdout},
		{name: "the named applet yields", override: "echo", stdout: overrideHelperStdout},
		{name: "one name in a list", override: "cat,echo ls", stdout: overrideHelperStdout},
		{name: "minus yields every applet", override: "-", stdout: overrideHelperStdout},
		{name: "plus yields where an external exists", override: "+", stdout: overrideHelperStdout},
		{name: "conditional with an external present", override: ";echo", stdout: overrideHelperStdout},
		{name: "conditional with no external of that name", override: ";cat", stdout: overrideAppletStdout},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// Given
			externalEchoOnPath(t)
			t.Setenv("NEMOSH_OVERRIDE_APPLETS", testCase.override)
			var stdout, stderr bytes.Buffer
			rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

			// When
			status := rt.RunScript(context.Background(), "echo "+overrideHelperArgs)

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

// The value is an ordinary shell variable, so a plain assignment takes effect
// and a later one changes its mind. busybox needs `export` here, because
// prefer_applet reads the process environment and ash only mirrors the variable
// into it on export (shell/ash.c:2976).
func TestRuntime_appliesTheOverride_whenTheScriptAssignsItWithoutExporting(t *testing.T) {
	// Given
	externalEchoOnPath(t)
	var stdout, stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "NEMOSH_OVERRIDE_APPLETS=echo\necho "+overrideHelperArgs+"NEMOSH_OVERRIDE_APPLETS=\necho back-to-the-applet\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d with stderr %q", status, stderr.String())
	}
	want := overrideHelperStdout + "back-to-the-applet\n"
	if got := stdout.String(); got != want {
		t.Fatalf("expected stdout %q, got %q", want, got)
	}
}

// An override with no external to fall back on is the user's mistake to make:
// busybox reports the same not-found the shell reports for any other unknown
// name, rather than quietly running the applet anyway.
func TestRuntime_reportsNotFound_whenAnOverriddenAppletHasNoExternal(t *testing.T) {
	// Given
	externalEchoOnPath(t)
	t.Setenv("NEMOSH_OVERRIDE_APPLETS", "-")
	var stdout, stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "cat /dev/null\n")

	// Then
	if status != 127 {
		t.Fatalf("expected status 127, got %d with stderr %q", status, stderr.String())
	}
	// The first line is the contract a script greps and does not move; the hint
	// after it is P1.1's and is allowed to say whatever is most useful.
	if got := stderr.String(); !strings.HasPrefix(got, "cat: not found\n") {
		t.Fatalf("expected stderr to start with %q, got %q", "cat: not found\n", got)
	}
}

// A shell builtin outranks an applet, so the override cannot reach one. This is
// busybox's ordering too: find_builtin runs before find_applet_by_name_for_sh
// (shell/ash.c:9840-9866).
func TestRuntime_keepsBuiltins_whenTheOverrideDisablesEveryApplet(t *testing.T) {
	// Given
	externalEchoOnPath(t)
	t.Setenv("NEMOSH_OVERRIDE_APPLETS", "-")
	var stdout, stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "export NEMOSH_OVERRIDE_PROBE=ok\nread -r line\n")

	// Then
	if status != 1 {
		t.Fatalf("expected status 1 from read at end of input, got %d with stderr %q", status, stderr.String())
	}
	if got := stderr.String(); got != "" {
		t.Fatalf("expected empty stderr, got %q", got)
	}
}

// `command -v` shares the one lookup, as it does in busybox where both go
// through find_command (shell/ash.c:9861), so it must not claim a name the
// shell would no longer dispatch to the applet.
func TestRuntime_stopsReportingAnAppletFromCommandV_whenItIsOverridden(t *testing.T) {
	// Given
	externalEchoOnPath(t)
	t.Setenv("NEMOSH_OVERRIDE_APPLETS", "cat")
	var stdout, stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "command -v cat\ncommand -v head\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0 from the second lookup, got %d with stderr %q", status, stderr.String())
	}
	if got := stdout.String(); got != "head\n" {
		t.Fatalf("expected only head to be reported, got %q", got)
	}
}
