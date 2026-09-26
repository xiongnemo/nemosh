package oilsspec_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/testutil/oilsspec"
)

// matrixPage is the page Matrix renders.
var matrixPage = filepath.Join("..", "..", "..", "docs", "testing", "oils-spec.md")

// docs/testing/oils-spec.md is what the data says: rendered again from baseline.json,
// calibration.json, the exclusions and the vendored cases, it is the same page. So a
// baseline or calibration that moves without the page fails here. NEMOSH_OILS_MATRIX=write
// renders it in place, as NEMOSH_OILS=update does after a run.
func TestMatrix_pageIsCurrent(t *testing.T) {
	page := renderMatrix(t)
	if os.Getenv("NEMOSH_OILS_MATRIX") == "write" {
		if err := os.WriteFile(matrixPage, []byte(page), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	committed, err := os.ReadFile(matrixPage)
	if err != nil {
		t.Fatal(err)
	}
	if string(committed) != page {
		t.Errorf("docs/testing/oils-spec.md is not what the data says; NEMOSH_OILS_MATRIX=write renders it again")
	}
}

// The page leads with the headline and holds a row for every file measured.
func TestMatrix_headlinesTheShareOfBashPassesAndListsEveryFile(t *testing.T) {
	page := renderMatrix(t)
	if !strings.Contains(page, "**nemosh passes ") || !strings.Contains(page, " cases bash passes: ") {
		t.Errorf("the page has no headline:\n%s", page)
	}
	for _, file := range []string{"alias.test.sh", "word-split.test.sh", "| all |"} {
		if !strings.Contains(page, "| "+strings.TrimPrefix(file, "| ")) {
			t.Errorf("the page has no row for %s", file)
		}
	}
}

func renderMatrix(t *testing.T) string {
	t.Helper()
	record := readUpstream(t)
	exclusions, err := oilsspec.ReadExclusions(root)
	if err != nil {
		t.Fatal(err)
	}
	calibration, err := oilsspec.ReadCalibration(root)
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := oilsspec.ReadBaseline(root)
	if err != nil {
		t.Fatal(err)
	}
	suite, err := oilsspec.LoadSuite(root, record, exclusions, baseline.Platform)
	if err != nil {
		t.Fatal(err)
	}
	return oilsspec.Matrix(suite, record, calibration, baseline)
}
