package oilsspec_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/xiongnemo/nemosh/internal/testutil/oilsspec"
)

// baseline.json records only cases nemosh does not pass, as failing or as done the way
// ash does, and each is a vendored case measured where the baseline was taken.
func TestBaseline_namesMeasuredCasesNemoshDoesNotPass(t *testing.T) {
	baseline, err := oilsspec.ReadBaseline(root)
	if err != nil {
		t.Fatal(err)
	}
	exclusions, err := oilsspec.ReadExclusions(root)
	if err != nil {
		t.Fatal(err)
	}
	specs := vendoredSpecs(t)
	for file, cases := range baseline.Cases {
		for id, standing := range cases {
			if standing != oilsspec.Fails && standing != oilsspec.AshOnly {
				t.Errorf("baseline.json has %s: %q as %q, want fail or ash-only", file, id, standing)
			}
			index := slices.IndexFunc(specs[file].Cases, func(c oilsspec.Case) bool { return c.ID == id })
			if index < 0 {
				t.Errorf("baseline.json names %s: %q, which is not a vendored case", file, id)
				continue
			}
			if rule, excluded := exclusions.Excluded(file, specs[file].Cases[index], baseline.Platform); excluded {
				t.Errorf("baseline.json names %s: %q, which %q leaves out on %s", file, id, rule.Name, baseline.Platform)
			}
		}
	}
}

// A run is compared with the baseline both ways: a case that stopped passing, one that
// started, one that changed how it fails, and one the baseline has that the run did not
// measure all differ, and a case that stands as recorded does not.
func TestBaseline_changesAreEveryDifferenceBothWays(t *testing.T) {
	baseline := oilsspec.Baseline{Platform: "windows", Cases: map[string]map[string]string{
		"a.test.sh": {"started passing": oilsspec.Fails, "still fails": oilsspec.Fails, "now fails outright": oilsspec.AshOnly},
		"b.test.sh": {"no longer measured": oilsspec.Fails},
	}}
	pass := oilsspec.CaseResult{Bash: oilsspec.Pass, Ash: oilsspec.Pass}
	fail := oilsspec.CaseResult{Bash: oilsspec.Fail, Ash: oilsspec.Fail}
	results := map[string][]oilsspec.CaseResult{"a.test.sh": {
		named(pass, "started passing"), named(fail, "still fails"), named(fail, "now fails outright"),
		named(fail, "stopped passing"), named(pass, "still passes"),
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
