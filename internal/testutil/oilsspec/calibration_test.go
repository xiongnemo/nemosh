package oilsspec_test

import (
	"slices"
	"testing"

	"github.com/xiongnemo/nemosh/internal/testutil/oilsspec"
)

// calibration.json holds bash, held to what the files record of bash, and busybox, held to
// what they record of ash, each with its version, and every case it names is a vendored
// case that was measured where it ran: one the exclusions leave out there cannot have been.
func TestCalibration_namesMeasuredCases(t *testing.T) {
	calibration, err := oilsspec.ReadCalibration(root)
	if err != nil {
		t.Fatal(err)
	}
	exclusions, err := oilsspec.ReadExclusions(root)
	if err != nil {
		t.Fatal(err)
	}
	specs := vendoredSpecs(t)
	for name, label := range map[string]string{"bash": "bash", "busybox": "ash"} {
		reference, found := calibration.Shells[name]
		if !found || reference.Label != label || reference.Version == "" {
			t.Errorf("calibration.json has %s as %+v, want it held to %s, with its version", name, reference, label)
			continue
		}
		for file, ids := range reference.Unmatched {
			for _, id := range ids {
				index := slices.IndexFunc(specs[file].Cases, func(c oilsspec.Case) bool { return c.ID == id })
				if index < 0 {
					t.Errorf("calibration.json has %s fail %s: %q, which is not a vendored case", name, file, id)
					continue
				}
				if rule, excluded := exclusions.Excluded(file, specs[file].Cases[index], calibration.Platform); excluded {
					t.Errorf("calibration.json has %s fail %s: %q, which %q leaves out on %s", name, file, id, rule.Name, calibration.Platform)
				}
			}
		}
	}
}
