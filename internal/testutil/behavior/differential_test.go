package behavior_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/testutil/behavior"
)

// loadShellCases reads every golden shell case that has something a reference
// could be asked about.
func loadShellCases(t *testing.T) []behavior.Case {
	t.Helper()
	root := filepath.Join("..", "..", "..", "tests", "behavior", "shell")
	var cases []behavior.Case
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".toml" {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		parsed, parseErr := behavior.ParseCase(data)
		if parseErr != nil {
			return parseErr
		}
		cases = append(cases, parsed)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return cases
}

// TestBehaviorDifferential_comparesGoldenCasesWithLocalReferences runs each
// golden shell case against the reference shells it names and reports where the
// reference disagrees with what the case expects.
//
// It reports rather than fails by default, for two reasons. A divergence may be
// a difference already recorded as deliberate in docs/design/v0-readiness.md,
// and the set of installed shells varies by machine, so failing here would make
// a green tree depend on the developer's toolbox. `NEMOSH_DIFFERENTIAL=strict`
// turns divergences into failures, which is what a machine with a known toolbox
// -- CI -- should do.
//
// Status and stdout only; see CompareWithReference for why stderr is left out.
func TestBehaviorDifferential_comparesGoldenCasesWithLocalReferences(t *testing.T) {
	mode := os.Getenv("NEMOSH_DIFFERENTIAL")
	strict := mode == "strict" || mode == "audit"
	executors := map[string]behavior.ScriptExecutor{}
	unavailable := map[string]string{}
	compared, skipped := 0, 0
	var report, stale []string

	for _, testCase := range loadShellCases(t) {
		// A case that pins a Nemosh extension has no reference to disagree with.
		if testCase.Script == "" || testCase.Kind != "golden" || testCase.Semantics == "nemosh" {
			continue
		}
		for _, name := range behavior.ComparableReferences(testCase) {
			if reason, seen := unavailable[name]; seen {
				_ = reason
				skipped++
				continue
			}
			executor, ready := executors[name]
			if !ready {
				resolved, reason := behavior.ReferenceExecutor(name)
				if reason != "" {
					unavailable[name] = reason
					skipped++
					continue
				}
				executors[name] = resolved
				executor = resolved
			}
			runner := behavior.NewRunnerWithScriptExecutor(applets.DefaultRegistry, executor)
			result := runner.Run(t.Context(), testCase)
			if result.SkipReason != "" {
				skipped++
				continue
			}
			if result.HarnessError != nil {
				report = append(report, testCase.ID+": "+name+": harness: "+result.HarnessError.Error())
				continue
			}
			compared++
			divergences := behavior.CompareWithReference(name, testCase.Expect,
				behavior.ProcessResult{Status: result.Status, Stdout: result.Stdout, Stderr: result.Stderr})
			if testCase.DivergenceDeclared(name) {
				if len(divergences) == 0 {
					stale = append(stale, testCase.ID+": "+name)
				}
				continue
			}
			for _, divergence := range divergences {
				report = append(report, testCase.ID+": "+divergence.String())
			}
		}
	}

	for name, reason := range unavailable {
		t.Logf("reference %s skipped: %s", name, reason)
	}
	t.Logf("compared %d case/reference pairs, skipped %d for want of a reference", compared, skipped)
	sort.Strings(report)
	for _, line := range report {
		if strict {
			t.Errorf("divergence: %s", line)
			continue
		}
		t.Logf("divergence: %s", line)
	}

	// A declared divergence that stopped happening is reported, and fails only
	// under `audit`. It cannot be a cross-platform gate: whether a reference
	// disagrees depends on which build of it is installed, and the first CI run
	// proved it -- Ubuntu's dash agrees with Nemosh on two cases where the dash
	// shipped with Git for Windows does not. Failing on that would mean the
	// declarations could only ever be right for one machine.
	//
	// `audit` is for a developer checking the list on a known toolbox, which is
	// the condition that makes the check meaningful.
	sort.Strings(stale)
	for _, line := range stale {
		if mode == "audit" {
			t.Errorf("declared divergence no longer happens, drop it from differential.diverges: %s", line)
			continue
		}
		t.Logf("declared divergence did not happen here (reference build differs?): %s", line)
	}
	if compared == 0 {
		t.Skip("no reference shell available on this machine")
	}
}

// A case citing a reference no adapter knows would be silently uncompared, which
// is the same failure shape as a test that never runs.
func TestBehaviorDifferential_everyCitedReferenceIsKnown(t *testing.T) {
	unknown := map[string][]string{}
	for _, testCase := range loadShellCases(t) {
		comparable := map[string]bool{}
		for _, name := range behavior.ComparableReferences(testCase) {
			comparable[name] = true
		}
		for _, name := range testCase.References {
			if comparable[name] || behavior.ProvenanceOnlyReference(name) {
				continue
			}
			unknown[name] = append(unknown[name], testCase.ID)
		}
	}
	for name, cases := range unknown {
		t.Errorf("reference %q has no adapter and is not recorded as provenance-only; cited by %s",
			name, strings.Join(cases, ", "))
	}
}
