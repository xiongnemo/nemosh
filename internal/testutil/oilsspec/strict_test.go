package oilsspec_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/testutil/oilsspec"
)

// reruns is how many times a case that stands otherwise than the baseline says is run
// again, alone, before the difference is believed. One that does not stand the same way
// every time is flaky, and is reported rather than failed.
const reruns = 2

// confirmChanges answers the cases the results stand otherwise for than the baseline says,
// each run again, one file at a time, until it stands the same way every time, and the ones
// that did not: the flaky. A case the baseline has and the run did not measure needs no
// rerun to be believed.
func confirmChanges(t *testing.T, suite oilsspec.Suite, subject oilsspec.Subject, work string, baseline oilsspec.Baseline, results map[string][]oilsspec.CaseResult) (confirmed, flaky []oilsspec.Change) {
	t.Helper()
	changes := baseline.Changes(results)
	rerun := map[string]map[string]bool{}
	for _, change := range changes {
		if change.Now == "" {
			continue
		}
		if rerun[change.File] == nil {
			rerun[change.File] = map[string]bool{}
		}
		rerun[change.File][change.ID] = true
	}
	unsteady := map[string]bool{}
	serial := suite
	serial.Parallel = 1
	for range reruns {
		if len(rerun) == 0 {
			break
		}
		again, err := serial.Run(context.Background(), subject, work, func(file, id string) bool { return rerun[file][id] })
		if err != nil {
			t.Fatal(err)
		}
		for _, change := range changes {
			for _, result := range again[change.File] {
				if result.ID == change.ID && oilsspec.Standing(result) != change.Now {
					unsteady[change.File+"\x00"+change.ID] = true
				}
			}
		}
	}
	for _, change := range changes {
		if unsteady[change.File+"\x00"+change.ID] {
			flaky = append(flaky, change)
		} else {
			confirmed = append(confirmed, change)
		}
	}
	return confirmed, flaky
}

// checkBaseline fails for every case that stands otherwise than baseline.json says, the one
// that started passing as much as the one that stopped, so the baseline is always where
// nemosh is. A flaky case is reported and does not fail.
func checkBaseline(t *testing.T, suite oilsspec.Suite, subject oilsspec.Subject, work string, results map[string][]oilsspec.CaseResult) {
	t.Helper()
	baseline, err := oilsspec.ReadBaseline(suite.Root)
	if err != nil {
		t.Fatal(err)
	}
	confirmed, flaky := confirmChanges(t, suite, subject, work, baseline, results)
	for _, change := range flaky {
		t.Logf("flaky, not failed: %s", describe(change))
	}
	if len(confirmed) == 0 {
		return
	}
	lines := make([]string, len(confirmed))
	for i, change := range confirmed {
		lines[i] = describe(change)
	}
	t.Errorf("%d cases stand otherwise than tests/oils/baseline.json says:\n%s\n"+
		"Once each is understood, NEMOSH_OILS=update records them.", len(confirmed), strings.Join(lines, "\n"))
}

// updateBaseline writes baseline.json from the run: a case that stands otherwise than it
// says, every time it is run, is recorded as it now stands, and a flaky one as it was.
func updateBaseline(t *testing.T, suite oilsspec.Suite, subject oilsspec.Subject, work, platform string, results map[string][]oilsspec.CaseResult) {
	t.Helper()
	baseline, err := oilsspec.ReadBaseline(suite.Root)
	if err != nil {
		baseline = oilsspec.Baseline{Cases: map[string]map[string]string{}}
	}
	baseline.Platform = platform
	confirmed, flaky := confirmChanges(t, suite, subject, work, baseline, results)
	for _, change := range confirmed {
		standing := change.Now
		if standing == "" {
			// Not measured any more, so there is nothing to record of it.
			standing = oilsspec.Passes
		}
		baseline.Set(change.File, change.ID, standing)
		t.Logf("recorded: %s", describe(change))
	}
	for _, change := range flaky {
		t.Logf("flaky, left as it was: %s", describe(change))
	}
	if err := baseline.Write(suite.Root); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(matrixPage, []byte(renderMatrix(t)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func describe(change oilsspec.Change) string {
	now := change.Now
	if now == "" {
		now = "not measured"
	}
	return fmt.Sprintf("%s: %q was %s, now %s", change.File, change.ID, change.Was, now)
}
