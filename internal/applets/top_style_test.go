package applets

import (
	"strings"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// The header before there is anything to put in it.
//
// The first sample has no rate, because a rate needs two, and what the header does about that is
// the first thing anyone sees. Drawing nothing left two blank lines that looked like a bug; drawing
// zeroes would look like an idle machine, which is a different claim from an unmeasured one.

func meterSnapshot(processors int) proc.Snapshot {
	return proc.Snapshot{
		Uptime: time.Hour,
		CPUs:   make([]proc.CPUTime, processors),
		Memory: proc.Memory{
			TotalPhysical: 1 << 34, AvailablePhysical: 1 << 33,
			CommitLimit: 1 << 34, CommitTotal: 1 << 33,
		},
	}
}

func TestTopSummary_theFirstSampleDrawsEmptyMetersAndSaysItIsSampling(t *testing.T) {
	// Interval zero is what proc.Between answers when there was no usable pair.
	rates := proc.Rates{Interval: 0}

	// When
	drawn := topSummaryText(meterSnapshot(4), rates, 100, 40)

	// Then -- a word for the reader
	if !strings.Contains(drawn, "sampling") {
		t.Fatalf("the first sample does not say it is sampling:\n%s", drawn)
	}
	// And one meter per processor, all of them empty
	for _, label := range []string{"0  ", "1  ", "2  ", "3  "} {
		if !strings.Contains(drawn, label) {
			t.Fatalf("no meter for processor %q:\n%s", strings.TrimSpace(label), drawn)
		}
	}
	// Empty, but only the meters that need a rate. Memory and commit are absolute counters and
	// are known on the first sample, so they draw real bars -- asserting that *nothing* is drawn
	// as busy caught those and was wrong about what the defect was.
	for _, line := range strings.Split(strings.TrimSpace(drawn), "\n") {
		if !strings.HasPrefix(line, "[white]CPU") && !strings.HasPrefix(line, "[white]0  ") {
			continue
		}
		if strings.Contains(line, "|") {
			t.Fatalf("a rate is drawn as busy before anything was measured: %q", line)
		}
		// Not as zero either, which would be wrong rather than absent.
		if strings.Contains(line, "0.0%") {
			t.Fatalf("an unmeasured rate is drawn as 0.0%%: %q", line)
		}
	}
	if !strings.Contains(drawn, "--") {
		t.Fatalf("nothing marks the figures as not yet known:\n%s", drawn)
	}
	// Memory is absolute and is known immediately, so it is *not* dashed out.
	if !strings.Contains(drawn, "50.0%") {
		t.Fatalf("memory should be known on the first sample:\n%s", drawn)
	}
}

// And the height does not change when the figures arrive, which is what would make the table jump.
func TestTopSummary_theHeaderIsTheSameHeightBeforeAndAfterTheFirstRate(t *testing.T) {
	snapshot := meterSnapshot(8)
	first := topSummaryText(snapshot, proc.Rates{}, 100, 40)
	busy := proc.Rates{Interval: time.Second, TotalBusy: 0.5, CPUs: make([]proc.CPURate, 8)}

	// When
	second := topSummaryText(snapshot, busy, 100, 40)

	// Then
	if got, want := strings.Count(first, "\n"), strings.Count(second, "\n"); got != want {
		t.Fatalf("the header is %d lines while sampling and %d after; the table would jump", got, want)
	}
	if got, want := topSummaryHeight(8, 100, 40), strings.Count(second, "\n"); got != want {
		t.Fatalf("the layout reserves %d lines for a header that draws %d", got, want)
	}
}
