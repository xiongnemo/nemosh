package oilsspec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
)

// Exclusions is tests/oils/exclusions.json: the cases the harness cannot measure on some
// platform, whatever the shell, and why. A rule names its cases by a pattern their code
// matches, such as chmod on Windows, where no program has an executable bit to set, or
// one by one. Only what cannot be measured is excluded: a case nemosh fails on purpose,
// because it refuses what the case asks, still counts.
type Exclusions struct {
	Rules []ExclusionRule `json:"rules"`
}

// ExclusionRule is one reason to leave cases out, and the platforms it holds on, as
// runtime.GOOS names them.
type ExclusionRule struct {
	Name      string              `json:"name"`
	Why       string              `json:"why"`
	Platforms []string            `json:"platforms"`
	Code      string              `json:"code,omitempty"`
	Cases     map[string][]string `json:"cases,omitempty"`
	pattern   *regexp.Regexp
}

// ReadExclusions reads exclusions.json from the vendored copy at root.
func ReadExclusions(root string) (Exclusions, error) {
	data, err := os.ReadFile(filepath.Join(root, "exclusions.json"))
	if err != nil {
		return Exclusions{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var exclusions Exclusions
	if err := decoder.Decode(&exclusions); err != nil {
		return Exclusions{}, fmt.Errorf("exclusions.json: %w", err)
	}
	for i, rule := range exclusions.Rules {
		if (rule.Code == "") == (len(rule.Cases) == 0) {
			return Exclusions{}, fmt.Errorf("exclusions.json: rule %s names its cases by neither a pattern nor a list, or by both", rule.Name)
		}
		if rule.Code != "" {
			pattern, err := regexp.Compile(rule.Code)
			if err != nil {
				return Exclusions{}, fmt.Errorf("exclusions.json: rule %s: %w", rule.Name, err)
			}
			exclusions.Rules[i].pattern = pattern
		}
	}
	return exclusions, nil
}

// Excluded answers the rule that leaves a case of file out on the platform, if one does.
func (e Exclusions) Excluded(file string, c Case, platform string) (ExclusionRule, bool) {
	for _, rule := range e.Rules {
		if !slices.Contains(rule.Platforms, platform) {
			continue
		}
		if rule.pattern != nil && rule.pattern.MatchString(c.Code) {
			return rule, true
		}
		if slices.Contains(rule.Cases[file], c.ID) {
			return rule, true
		}
	}
	return ExclusionRule{}, false
}
