package oilsspec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
)

// Calibration is tests/oils/calibration.json: what the references did with the cases, run
// through this harness. Bash's answers are the headline's denominator, since a case bash
// itself fails here -- Oils recorded its expectations with an older bash, on Linux -- says
// nothing about how far nemosh is from bash. Both are also the check that the harness is
// faithful: bash run through it should do what the files record of bash.
type Calibration struct {
	// Platform is where the references ran, as runtime.GOOS names it.
	Platform string `json:"platform"`
	// Measured is the day they ran.
	Measured string `json:"measured"`
	// Shells are bash, held to what the files record of bash, and busybox, held to what
	// they record of ash.
	Shells map[string]Reference `json:"shells"`
}

// Reference is one reference's calibration: the shell as it names itself, and the cases
// it did not do what the files record of it, by file.
type Reference struct {
	Version   string              `json:"version"`
	Label     string              `json:"label"`
	Unmatched map[string][]string `json:"unmatched"`
}

// ReadCalibration reads calibration.json from the vendored copy at root.
func ReadCalibration(root string) (Calibration, error) {
	data, err := os.ReadFile(filepath.Join(root, "calibration.json"))
	if err != nil {
		return Calibration{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var calibration Calibration
	if err := decoder.Decode(&calibration); err != nil {
		return Calibration{}, fmt.Errorf("calibration.json: %w", err)
	}
	return calibration, nil
}

// Write writes the calibration to root/calibration.json, its cases in order, so a new one
// reads as a diff of the old.
func (c Calibration) Write(root string) error {
	for _, shell := range c.Shells {
		for _, ids := range shell.Unmatched {
			sort.Strings(ids)
		}
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "calibration.json"), append(data, '\n'), 0o644)
}

// Passes reports whether the reference did what the files record of it with a case.
func (r Reference) Passes(file, id string) bool {
	return !slices.Contains(r.Unmatched[file], id)
}

// Unmatched gathers, from a reference's results, the cases it did not match what the
// files record of its label.
func Unmatched(label string, results map[string][]CaseResult) map[string][]string {
	unmatched := map[string][]string{}
	for file, fileResults := range results {
		for _, result := range fileResults {
			scored := result.Bash
			if label == "ash" {
				scored = result.Ash
			}
			if !scored.Matched() {
				unmatched[file] = append(unmatched[file], result.ID)
			}
		}
	}
	return unmatched
}
