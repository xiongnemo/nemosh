package oilsspec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// How a case stands for nemosh: it does what the files record of bash, it does what they
// record of ash instead, as busybox might, or it does neither.
const (
	Passes  = "pass"
	AshOnly = "ash-only"
	Fails   = "fail"
)

// Standing is how a result stands, in those terms.
func Standing(result CaseResult) string {
	switch {
	case result.Bash.Matched():
		return Passes
	case result.Ash.Matched():
		return AshOnly
	}
	return Fails
}

// Baseline is tests/oils/baseline.json: every measured case nemosh does not pass, and how
// it stands, by file. A case not in it passes. A run that differs from it fails the strict
// mode, the case that started passing as much as the one that stopped, so the baseline
// always says where nemosh is.
type Baseline struct {
	Platform string                       `json:"platform"`
	Cases    map[string]map[string]string `json:"cases"`
}

// ReadBaseline reads baseline.json from the vendored copy at root.
func ReadBaseline(root string) (Baseline, error) {
	data, err := os.ReadFile(filepath.Join(root, "baseline.json"))
	if err != nil {
		return Baseline{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var baseline Baseline
	if err := decoder.Decode(&baseline); err != nil {
		return Baseline{}, fmt.Errorf("baseline.json: %w", err)
	}
	return baseline, nil
}

// Write writes the baseline to root/baseline.json, in order, so a new one reads as a diff
// of the old.
func (b Baseline) Write(root string) error {
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, "baseline.json"), append(data, '\n'), 0o644)
}

// NewBaseline records how each result stands.
func NewBaseline(platform string, results map[string][]CaseResult) Baseline {
	baseline := Baseline{Platform: platform, Cases: map[string]map[string]string{}}
	for file, fileResults := range results {
		for _, result := range fileResults {
			baseline.Set(file, result.ID, Standing(result))
		}
	}
	return baseline
}

// Standing is how the baseline says a case stands.
func (b Baseline) Standing(file, id string) string {
	if standing, recorded := b.Cases[file][id]; recorded {
		return standing
	}
	return Passes
}

// Set records how a case stands.
func (b Baseline) Set(file, id, standing string) {
	if standing == Passes {
		delete(b.Cases[file], id)
		if len(b.Cases[file]) == 0 {
			delete(b.Cases, file)
		}
		return
	}
	if b.Cases[file] == nil {
		b.Cases[file] = map[string]string{}
	}
	b.Cases[file][id] = standing
}

// Change is a case that stands otherwise than the baseline says. Now is empty for a case
// the baseline records and the run did not measure.
type Change struct {
	File, ID, Was, Now string
}

// Changes lists, in order, the cases the results stand otherwise for than the baseline
// says, and the ones it records that the results do not have.
func (b Baseline) Changes(results map[string][]CaseResult) []Change {
	var changes []Change
	measured := map[string]map[string]bool{}
	for file, fileResults := range results {
		measured[file] = map[string]bool{}
		for _, result := range fileResults {
			measured[file][result.ID] = true
			if was, now := b.Standing(file, result.ID), Standing(result); was != now {
				changes = append(changes, Change{File: file, ID: result.ID, Was: was, Now: now})
			}
		}
	}
	for file, cases := range b.Cases {
		for id, was := range cases {
			if !measured[file][id] {
				changes = append(changes, Change{File: file, ID: id, Was: was})
			}
		}
	}
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].File != changes[j].File {
			return changes[i].File < changes[j].File
		}
		return changes[i].ID < changes[j].ID
	})
	return changes
}
