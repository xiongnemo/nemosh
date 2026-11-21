package oilsspec_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/xiongnemo/nemosh/internal/testutil/oilsspec"
)

// Each baseline -- baseline.json for nemosh run as bash, baseline-sh.json for it run as sh --
// records only cases nemosh does not pass -- failing, done the way ash does, or flaky -- and
// each is a vendored case measured where the baseline was taken.
func TestBaseline_namesMeasuredCasesNemoshDoesNotPass(t *testing.T) {
	exclusions, err := oilsspec.ReadExclusions(root)
	if err != nil {
		t.Fatal(err)
	}
	specs := vendoredSpecs(t)
	for _, as := range []string{"bash", "sh"} {
		name := oilsspec.BaselineFile(as)
		baseline, err := oilsspec.ReadBaseline(root, name)
		if err != nil {
			t.Error(err)
			continue
		}
		for file, cases := range baseline.Cases {
			for id, standing := range cases {
				if standing != oilsspec.Fails && standing != oilsspec.AshOnly && standing != oilsspec.Flaky {
					t.Errorf("%s has %s: %q as %q, want fail, ash-only or flaky", name, file, id, standing)
				}
				index := slices.IndexFunc(specs[file].Cases, func(c oilsspec.Case) bool { return c.ID == id })
				if index < 0 {
					t.Errorf("%s names %s: %q, which is not a vendored case", name, file, id)
					continue
				}
				if rule, excluded := exclusions.Excluded(file, specs[file].Cases[index], baseline.Platform); excluded {
					t.Errorf("%s names %s: %q, which %q leaves out on %s", name, file, id, rule.Name, baseline.Platform)
				}
			}
		}
	}
}

// The two names keep two files: bash's is the headline's, and sh's holds busybox's name to
// what it does while bash's learns bash's ways.
func TestBaselineFile_isOnePerName(t *testing.T) {
	for as, want := range map[string]string{"bash": "baseline.json", "": "baseline.json", "sh": "baseline-sh.json"} {
		if got := oilsspec.BaselineFile(as); got != want {
			t.Errorf("BaselineFile(%q) = %q, want %q", as, got, want)
		}
	}
}

// A run is compared with the baseline both ways: a case that stopped passing, one that
// started, one that changed how it fails, and one the baseline has that the run did not
// measure all differ, and a case that stands as recorded does not -- nor one recorded as
// flaky, however it came out.
func TestBaseline_changesAreEveryDifferenceBothWays(t *testing.T) {
	baseline := oilsspec.Baseline{Platform: "windows", Cases: map[string]map[string]string{
		"a.test.sh": {"started passing": oilsspec.Fails, "still fails": oilsspec.Fails, "now fails outright": oilsspec.AshOnly,
			"flaky, passed": oilsspec.Flaky, "flaky, failed": oilsspec.Flaky},
		"b.test.sh": {"no longer measured": oilsspec.Fails},
	}}
	pass := oilsspec.CaseResult{Bash: oilsspec.Pass, Ash: oilsspec.Pass}
	fail := oilsspec.CaseResult{Bash: oilsspec.Fail, Ash: oilsspec.Fail}
	results := map[string][]oilsspec.CaseResult{"a.test.sh": {
		named(pass, "started passing"), named(fail, "still fails"), named(fail, "now fails outright"),
		named(fail, "stopped passing"), named(pass, "still passes"),
		named(pass, "flaky, passed"), named(fail, "flaky, failed"),
	}}

	changes := baseline.Changes(results)

	want := []oilsspec.Change{
		{File: "a.test.sh", ID: "now fails outright", Was: oilsspec.AshOnly, Now: oilsspec.Fails},
		{File: "a.test.sh", ID: "started passing", Was: oilsspec.Fails, Now: oilsspec.Passes},
		{File: "a.test.sh", ID: "stopped passing", Was: oilsspec.Passes, Now: oilsspec.Fails},
		{File: "b.test.sh", ID: "no longer measured", Was: oilsspec.Fails},
	}
	if !reflect.DeepEqual(changes, want) {
		t.Errorf("changes\n got %+v\nwant %+v", changes, want)
	}
}

// Recording changes is what update writes and what strict proposes: a case that now passes
// leaves the baseline, one that stands otherwise is set as it stands, and one no longer
// measured has nothing left to record.
func TestBaseline_recordsChangesAsTheyNowStand(t *testing.T) {
	baseline := oilsspec.Baseline{Platform: "windows", Cases: map[string]map[string]string{
		"a.test.sh": {"started passing": oilsspec.Fails, "now ash-only": oilsspec.Fails, "unchanged": oilsspec.Fails},
		"b.test.sh": {"no longer measured": oilsspec.Fails},
	}}
	baseline.Record([]oilsspec.Change{
		{File: "a.test.sh", ID: "started passing", Was: oilsspec.Fails, Now: oilsspec.Passes},
		{File: "a.test.sh", ID: "now ash-only", Was: oilsspec.Fails, Now: oilsspec.AshOnly},
		{File: "a.test.sh", ID: "stopped passing", Was: oilsspec.Passes, Now: oilsspec.Fails},
		{File: "b.test.sh", ID: "no longer measured", Was: oilsspec.Fails},
	})
	want := map[string]map[string]string{
		"a.test.sh": {"now ash-only": oilsspec.AshOnly, "unchanged": oilsspec.Fails, "stopped passing": oilsspec.Fails},
	}
	if !reflect.DeepEqual(baseline.Cases, want) {
		t.Errorf("recorded\n got %v\nwant %v", baseline.Cases, want)
	}
}

// A result stands by what it matched: bash's expectations, else ash's, else neither. A
// timeout matches nothing.
func TestStanding_isBashFirstThenAsh(t *testing.T) {
	tests := []struct {
		bash, ash oilsspec.Result
		want      string
	}{
		{oilsspec.Pass, oilsspec.Fail, oilsspec.Passes},
		{oilsspec.NotImplemented, oilsspec.Pass, oilsspec.Passes},
		{oilsspec.Fail, oilsspec.OK, oilsspec.AshOnly},
		{oilsspec.Fail, oilsspec.Fail, oilsspec.Fails},
		{oilsspec.Timeout, oilsspec.Timeout, oilsspec.Fails},
	}
	for _, test := range tests {
		if got := oilsspec.Standing(oilsspec.CaseResult{Bash: test.bash, Ash: test.ash}); got != test.want {
			t.Errorf("bash %v, ash %v stands as %q, want %q", test.bash, test.ash, got, test.want)
		}
	}
}

func named(result oilsspec.CaseResult, id string) oilsspec.CaseResult {
	result.ID = id
	return result
}
