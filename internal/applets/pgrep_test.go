package applets_test

import (
	"errors"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/proc"
)

// A pattern that matches nothing must be refused before anything is listed, and
// an *empty* pattern must be refused outright: it would match every process on
// the machine, and for pkill that is not a mistake anyone recovers from.
func TestPgrep_refusesWhatWouldMatchEverything(t *testing.T) {
	for _, test := range []struct {
		applet string
		args   []string
		want   string
	}{
		{applet: "pgrep", args: []string{""}, want: "every process"},
		{applet: "pkill", args: []string{""}, want: "every process"},
		{applet: "pgrep", args: nil, want: "missing operand"},
		{applet: "pkill", args: nil, want: "missing operand"},
		{applet: "pgrep", args: []string{"a", "b"}, want: "extra operand"},
		{applet: "pgrep", args: []string{"("}, want: "invalid pattern"},
		// -f would have to read each process's command line, which needs
		// privileges an ordinary session has not got, so it is refused rather
		// than quietly matching the name instead.
		{applet: "pgrep", args: []string{"-f", "x"}, want: "invalid option"},
	} {
		t.Run(test.applet+" "+strings.Join(test.args, " "), func(t *testing.T) {
			_, stderr, err := runAppletWithInput(t, "", test.applet, test.args...)
			reported := stderr
			if err != nil {
				reported += err.Error()
			}
			if !strings.Contains(reported, test.want) {
				t.Fatalf("%s %v reported %q, want it to mention %q", test.applet, test.args, reported, test.want)
			}
		})
	}
}

// Nothing matching is an answer, not a failure -- status 1 with no diagnostic, so
// `pgrep foo >/dev/null` is a usable test.
func TestPgrep_reportsNoMatchAsAStatus(t *testing.T) {
	if _, ok := listableOrSkip(t); !ok {
		return
	}
	stdout, stderr, err := runAppletWithInput(t, "", "pgrep", "zzznosuchprocessname")
	if stdout != "" || stderr != "" {
		t.Fatalf("stdout = %q, stderr = %q, want both empty", stdout, stderr)
	}
	status, ok := applets.StatusCode(err)
	if !ok || status != 1 {
		t.Fatalf("status = %d (recognised %v), want 1", status, ok)
	}
}

// pgrep finds a process this test starts, by the name a person would type -- with
// no executable suffix, because `pkill notepad` is what anyone writes for
// `notepad.exe`.
func TestPgrep_findsAProcessByTheNameYouType(t *testing.T) {
	if _, ok := listableOrSkip(t); !ok {
		return
	}
	// Given: a child of our own binary, whose name we therefore know
	binary := buildHelperShell(t)
	child := exec.Command(binary, "-c", "sleep 60")
	if err := child.Start(); err != nil {
		t.Fatalf("starting the helper: %v", err)
	}
	defer func() { _ = child.Process.Kill() }()

	name := "helper"
	deadline := time.Now().Add(10 * time.Second)
	for {
		stdout, _, err := runAppletWithInput(t, "", "pgrep", "-l", name)
		if err == nil && strings.Contains(stdout, strconv.Itoa(child.Process.Pid)) {
			// And -l names it, which is the whole difference from bare pgrep.
			if !strings.Contains(strings.ToLower(stdout), "helper") {
				t.Fatalf("pgrep -l = %q, want the process name too", stdout)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("pgrep never found pid %d; last output %q err %v", child.Process.Pid, stdout, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// pkill stops what pgrep found. Tested only against a process this test started.
func TestPkill_stopsWhatItMatches(t *testing.T) {
	if _, ok := listableOrSkip(t); !ok {
		return
	}
	// Given
	binary := buildHelperShell(t)
	child := exec.Command(binary, "-c", "sleep 60")
	if err := child.Start(); err != nil {
		t.Fatalf("starting the helper: %v", err)
	}
	defer func() { _ = child.Process.Kill() }()
	waitFor(t, func() bool {
		_, _, err := runAppletWithInput(t, "", "pgrep", "helper")
		return err == nil
	}, "pgrep never saw the helper")

	// When: the name is exact, so nothing else on the machine can match
	if _, stderr, err := runAppletWithInput(t, "", "pkill", "-x", "helper"); err != nil {
		t.Fatalf("pkill = %v, stderr = %q", err, stderr)
	}

	// Then
	stopped := make(chan error, 1)
	go func() { stopped <- child.Wait() }()
	select {
	case <-stopped:
	case <-time.After(20 * time.Second):
		t.Fatal("the process survived pkill")
	}
}

// The process list is what these need, and where it cannot be read they say so
// rather than answering "no match" on a machine full of processes.
func listableOrSkip(t *testing.T) ([]proc.Process, bool) {
	t.Helper()
	processes, err := proc.List()
	if errors.Is(err, proc.ErrListUnsupported) {
		t.Skipf("the process list is not implemented on %s, which is a build-and-test target", runtime.GOOS)
		return nil, false
	}
	if err != nil {
		t.Fatalf("listing processes: %v", err)
	}
	return processes, true
}

func buildHelperShell(t *testing.T) string {
	t.Helper()
	binary := t.TempDir() + "/helper"
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-o", binary, "github.com/xiongnemo/nemosh/cmd/nemosh")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the helper: %v\n%s", err, output)
	}
	return binary
}

func waitFor(t *testing.T, condition func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal(message)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
