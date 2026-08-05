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
	return problems
}

func safeRelativePath(path string) bool {
	return path != "" && path != "." && filepath.IsLocal(filepath.FromSlash(path))
}
