//go:build windows

package applets

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The child. It runs only when the parent asked for it, writes where it was
// started and what it was given, and exits with a status nothing else would
// produce by accident.
func TestSuHelperProcess(t *testing.T) {
	if os.Getenv("NEMOSH_SU_HELPER") != "1" {
		return
	}
	var target string
	for index, argument := range os.Args {
		if argument == "--" && index+1 < len(os.Args) {
			target = os.Args[index+1]
		}
	}
	if target == "" {
		os.Exit(9)
	}
	cwd, err := os.Getwd()
	if err != nil {
		os.Exit(8)
	}
	if err := os.WriteFile(target, []byte(cwd+"\n"+strings.Join(os.Args, "\x00")), 0o600); err != nil {
		os.Exit(7)
	}
	os.Exit(3)
}

// The launch path, end to end, without elevating anything.
//
// This is what `-t` is for and why busybox has it (suw32.c:88-91): the verb
// becomes `open` instead of `runas`, so ShellExecuteEx, the quoting, the working
// directory, the wait and the exit status are all exercised and no consent
// dialog appears. Everything here would be untestable otherwise, since a UAC
// prompt cannot be answered by a test.
func TestRunElevated_launchesWaitsAndReportsStatus(t *testing.T) {
	// Given
	t.Setenv("NEMOSH_SU_HELPER", "1")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	report := filepath.Join(t.TempDir(), "report.txt")
	plan := elevationPlan{
		program: executable,
		// An argument with a space in it, because the vector has to survive
		// being flattened into one string and split again by the child.
		arguments: joinWindowsArguments([]string{"-test.run=TestSuHelperProcess", "--", report, "a b"}),
		directory: directory,
		wait:      true,
		test:      true,
	}

	// When
	err = runElevated(context.Background(), plan)

	// Then: the child's status, carried back rather than swallowed.
	status, ok := StatusCode(err)
	if !ok || status != 3 {
		t.Fatalf("runElevated = %v, want exit status 3", err)
	}
	written, readErr := os.ReadFile(report)
	if readErr != nil {
		t.Fatalf("the child wrote no report: %v", readErr)
	}
	lines := strings.SplitN(string(written), "\n", 2)
	// The working directory reaches the child. ShellExecuteEx would otherwise
	// decide for itself, and busybox's comment says it picks System32 for a
	// program in a system directory (suw32.c:96-102).
	if !sameDirectory(lines[0], directory) {
		t.Fatalf("child started in %q, want %q", lines[0], directory)
	}
	// And the argument with a blank in it arrived as one argument.
	if !strings.Contains(lines[1], "\x00a b") {
		t.Fatalf("child argv = %q, want an argument %q", lines[1], "a b")
	}
}

// Not waiting is a different path: no handle is kept, so nothing can be
// reported, and su has to return success rather than invent one.
func TestRunElevated_returnsImmediatelyWithoutWait(t *testing.T) {
	// Given
	t.Setenv("NEMOSH_SU_HELPER", "1")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(t.TempDir(), "report.txt")
	plan := elevationPlan{
		program:   executable,
		arguments: joinWindowsArguments([]string{"-test.run=TestSuHelperProcess", "--", report}),
		directory: t.TempDir(),
		test:      true,
	}

	// When
	err = runElevated(context.Background(), plan)

	// Then
	if err != nil {
		t.Fatalf("runElevated = %v, want nil: without -W there is nothing to report", err)
	}
}

func sameDirectory(left, right string) bool {
	return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
}
