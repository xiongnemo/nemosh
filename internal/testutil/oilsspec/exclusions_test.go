package oilsspec_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/testutil/oilsspec"
)

// Every exclusion says why, on platforms Go names, and leaves out something that is
// there: a rule's pattern matches some vendored case, and a case it names is a case of
// the file it names. A refresh of the vendored copy that drops a case, or a rule whose
// pattern stopped matching, fails here rather than excluding nothing in silence.
func TestExclusions_leaveOutRealCasesForAReason(t *testing.T) {
	exclusions, err := oilsspec.ReadExclusions(root)
	if err != nil {
		t.Fatal(err)
	}
	specs := vendoredSpecs(t)
	for _, rule := range exclusions.Rules {
		if rule.Why == "" || len(rule.Platforms) == 0 {
			t.Errorf("rule %q gives no reason or no platform", rule.Name)
		}
		for _, platform := range rule.Platforms {
			if !slices.Contains([]string{"windows", "linux", "darwin"}, platform) {
				t.Errorf("rule %q names platform %q, which Go does not", rule.Name, platform)
			}
		}
		matched := 0
		for file, spec := range specs {
			for _, c := range spec.Cases {
				if found, excluded := exclusions.Excluded(file, c, rule.Platforms[0]); excluded && found.Name == rule.Name {
					matched++
				}
			}
		}
		if matched == 0 {
			t.Errorf("rule %q leaves out no vendored case", rule.Name)
		}
		for file, ids := range rule.Cases {
			for _, id := range ids {
				if !slices.ContainsFunc(specs[file].Cases, func(c oilsspec.Case) bool { return c.ID == id }) {
					t.Errorf("rule %q names %s: %q, which is not a vendored case", rule.Name, file, id)
				}
			}
		}
	}
}

// A rule leaves a case out on the platforms it names and nowhere else, by its code or
// by its name.
func TestExclusions_holdOnlyWhereTheySay(t *testing.T) {
	exclusions, err := oilsspec.ReadExclusions(root)
	if err != nil {
		t.Fatal(err)
	}
	chmod := oilsspec.Case{ID: "made executable", Code: "chmod +x f\n./f\n"}
	home := oilsspec.Case{ID: "$HOME is NOT set", Code: "echo $HOME\n"}
	tests := []struct {
		file     string
		c        oilsspec.Case
		platform string
		want     string
	}{
		{"x.test.sh", chmod, "windows", "executable bits"},
		{"x.test.sh", chmod, "linux", ""},
		{"vars-special.test.sh", home, "linux", "HOME"},
		{"vars-special.test.sh", home, "windows", "HOME"},
		{"other.test.sh", home, "linux", ""},
	}
	for _, test := range tests {
		rule, excluded := exclusions.Excluded(test.file, test.c, test.platform)
		if got := map[bool]string{true: rule.Name}[excluded]; got != test.want {
			t.Errorf("%s %q on %s: excluded by %q, want %q", test.file, test.c.ID, test.platform, got, test.want)
		}
	}
}

// vendoredSpecs parses every vendored spec file, keyed by its name in spec/.
func vendoredSpecs(t *testing.T) map[string]oilsspec.Spec {
	t.Helper()
	specs := map[string]oilsspec.Spec{}
	for name := range readUpstream(t).Files {
		if !strings.HasSuffix(name, ".test.sh") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		spec, err := oilsspec.Parse(string(data))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		specs[strings.TrimPrefix(name, "spec/")] = spec
	}
	return specs
}
