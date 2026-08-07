package behavior

import (
	"fmt"
	"path/filepath"
)

type Case struct {
	ID         string            `toml:"id"`
	Area       string            `toml:"area"`
	Kind       string            `toml:"kind"`
	Semantics  string            `toml:"semantics"`
	Platforms  []string          `toml:"platforms"`
	References []string          `toml:"references"`
	Script     string            `toml:"script"`
	Command    []string          `toml:"command"`
	Stdin      string            `toml:"stdin"`
	CWD        string            `toml:"cwd"`
	Env        map[string]string `toml:"env"`
	Files      map[string]string `toml:"files"`
	Requires   []string          `toml:"requires"`
	Expect     Expect            `toml:"expect"`
	Notes      Notes             `toml:"notes"`
	// Differential records where a reference shell is expected to disagree, so
	// the comparison can be strict everywhere else.
	Differential Differential `toml:"differential"`
}

// Differential names the references whose answer to this case is expected to
// differ from Nemosh's, and why.
//
// A declared divergence is checked in both directions: the runner does not fail
// on it, and it *does* fail when the divergence stops happening. Otherwise the
// list rots into a set of exemptions nobody revisits, which is how a
// known-failures file becomes a place bugs hide.
type Differential struct {
	Diverges []string `toml:"diverges"`
	Why      string   `toml:"why"`
}

// DivergenceDeclared reports whether this case expects the named reference to
// disagree.
func (c Case) DivergenceDeclared(reference string) bool {
	for _, name := range c.Differential.Diverges {
		if name == reference {
			return true
		}
	}
	return false
}

type Expect struct {
	Status int    `toml:"status"`
	Stdout string `toml:"stdout"`
	Stderr string `toml:"stderr"`
}

type Notes struct {
	Standard string `toml:"standard"`
	Why      string `toml:"why"`
}

func (c Case) Validate() []string {
	var problems []string
	if c.ID == "" {
		problems = append(problems, "missing id")
	}
	if c.Area == "" {
		problems = append(problems, "missing area")
	}
	if c.Kind == "" {
		problems = append(problems, "missing kind")
	}
	if c.Semantics == "" {
		problems = append(problems, "missing semantics")
	}
	if len(c.Platforms) == 0 {
		problems = append(problems, "missing platforms")
	}
	if (c.Script == "") == (len(c.Command) == 0) {
		problems = append(problems, "exactly one of script or command is required")
	}
	if c.CWD != "" && !safeRelativePath(c.CWD) {
		problems = append(problems, fmt.Sprintf("unsafe cwd %q", c.CWD))
	}
	for path := range c.Files {
		if !safeRelativePath(path) {
			problems = append(problems, fmt.Sprintf("unsafe file path %q", path))
		}
	}
	if len(c.Differential.Diverges) > 0 && c.Differential.Why == "" {
		problems = append(problems, "differential.diverges requires differential.why")
	}
	return problems
}

func safeRelativePath(path string) bool {
	return path != "" && path != "." && filepath.IsLocal(filepath.FromSlash(path))
}
