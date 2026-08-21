//go:build windows

package proc_test

import (
	"os"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// Changing a priority, against this test's own process.
//
// Its own, because that is the only process a test can be sure it may open: the interesting half of
// this feature on Windows is that most processes refuse, and a test that picked one at random would
// pass or fail depending on who owned it. It puts the priority back, though a test binary exits
// moments later anyway.

func TestAdjustPriority_stepsUpAndDown(t *testing.T) {
	pid := os.Getpid()

	// When -- one step up
	raised, err := proc.AdjustPriority(pid, 1)
	if err != nil {
		t.Fatalf("raising this process: %v", err)
	}
	if raised == "" {
		t.Fatal("no class was reported")
	}
	t.Cleanup(func() { _, _ = proc.AdjustPriority(pid, -1) })

	// Then -- and one step back down lands somewhere with a name
	lowered, err := proc.AdjustPriority(pid, -1)
	if err != nil {
		t.Fatalf("lowering it again: %v", err)
	}
	if lowered == raised {
		t.Fatalf("a step down from %s reported %s", raised, lowered)
	}
	// The names are the ones the PRI column shows, so a reader sees the same word in both
	// places rather than a number here and a word there.
	for _, name := range []string{raised, lowered} {
		switch name {
		case "idle", "below", "norm", "above", "high", "real":
		default:
			t.Fatalf("reported %q, which is not a name the PRI column uses", name)
		}
	}
}

// A process this session does not own is refused, with the reason rather than a Win32 code. This is
// the common case on Windows and the one a person meets first.
func TestAdjustPriority_refusesWhatItCannotOpen(t *testing.T) {
	// PID 4 is System, which no session owns.
	_, err := proc.AdjustPriority(4, 1)
	if err == nil {
		t.Fatal("changing System's priority was allowed")
	}
	if !strings.Contains(err.Error(), "does not own") {
		t.Fatalf("refused with %v, which does not say why", err)
	}
}

func TestAdjustPriority_refusesANonProcess(t *testing.T) {
	if _, err := proc.AdjustPriority(0, 1); err == nil {
		t.Fatal("pid 0 was accepted")
	}
	if _, err := proc.AdjustPriority(-1, 1); err == nil {
		t.Fatal("a negative pid was accepted")
	}
}
