package oilsspec

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// Suite is the vendored spec files, parsed, less what this platform cannot measure.
type Suite struct {
	// Root is the vendored copy, tests/oils.
	Root   string
	record Upstream
	// Specs are the files to run, by name in spec/, each with only its measured cases.
	Specs map[string]Spec
	// Left counts the cases left out, by the exclusion rule that left them out.
	Left map[string]int
	// Disabled counts the cases in files Oils marks `suite: disabled`.
	Disabled int
	// Parallel is how many files Run runs at once: as many as there are processors when
	// it is zero.
	Parallel int
}

// LoadSuite parses every vendored spec file and leaves out what the exclusions leave out on
// the platform. A file Oils marks `suite: disabled` is left out whole, as Oils leaves it.
func LoadSuite(root string, record Upstream, exclusions Exclusions, platform string) (Suite, error) {
	suite := Suite{Root: root, record: record, Specs: map[string]Spec{}, Left: map[string]int{}}
	for name := range record.Files {
		if !strings.HasSuffix(name, ".test.sh") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			return Suite{}, err
		}
		spec, err := Parse(string(data))
		if err != nil {
			return Suite{}, fmt.Errorf("%s: %w", name, err)
		}
		if spec.Metadata["suite"] == "disabled" {
			suite.Disabled += len(spec.Cases)
			continue
		}
		file := strings.TrimPrefix(name, "spec/")
		var measured []Case
		for _, c := range spec.Cases {
			if rule, excluded := exclusions.Excluded(file, c, platform); excluded {
				suite.Left[rule.Name]++
				continue
			}
			measured = append(measured, c)
		}
		spec.Cases = measured
		suite.Specs[file] = spec
	}
	return suite, nil
}

// Run runs the suite with the subject under work: the files side by side, as many at once
// as Parallel says, and the cases of each in order, each file with a $REPO_ROOT of its
// own, since cases write into it. When only is not nil, it runs just the cases only
// answers true for, and the files that have one.
func (s Suite) Run(ctx context.Context, subject Subject, work string, only func(file, id string) bool) (map[string][]CaseResult, error) {
	results := map[string][]CaseResult{}
	var mu sync.Mutex
	var group sync.WaitGroup
	var failures []error
	parallel := s.Parallel
	if parallel == 0 {
		parallel = runtime.NumCPU()
	}
	slots := make(chan struct{}, parallel)
	for file, spec := range s.Specs {
		if only != nil {
			var chosen []Case
			for _, c := range spec.Cases {
				if only(file, c.ID) {
					chosen = append(chosen, c)
				}
			}
			if len(chosen) == 0 {
				continue
			}
			spec.Cases = chosen
		}
		group.Add(1)
		go func() {
			defer group.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			fileResults, err := s.runFile(ctx, subject, file, spec, work)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failures = append(failures, fmt.Errorf("%s: %w", file, err))
				return
			}
			results[file] = fileResults
		}()
	}
	group.Wait()
	if len(failures) > 0 {
		return results, failures[0]
	}
	return results, nil
}

func (s Suite) runFile(ctx context.Context, subject Subject, file string, spec Spec, work string) ([]CaseResult, error) {
	dir, err := os.MkdirTemp(work, strings.TrimSuffix(file, ".test.sh")+"-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	repo := filepath.Join(dir, "repo")
	if err := s.record.RepoRoot(s.Root, repo); err != nil {
		return nil, err
	}
	return subject.RunSpec(ctx, spec, filepath.Join(dir, "cases"), repo)
}
