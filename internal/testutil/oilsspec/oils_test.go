package oilsspec_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/xiongnemo/nemosh/internal/testutil/oilsspec"
)

// TestOilsSpec runs the vendored Oils spec suite with nemosh and says how far nemosh is
// from bash: how many cases do what the files record of bash, and how many what they
// record of ash, which is busybox's shell. It takes minutes, so it runs only when asked:
//
//	NEMOSH_OILS=report go test ./internal/testutil/oilsspec/ -run TestOilsSpec -count=1 -timeout 30m
//
// NEMOSH_OILS=calibrate runs the references instead, bash from NEMOSH_OILS_BASH and
// busybox from NEMOSH_OILS_BUSYBOX, which is how the harness is shown to be faithful:
// bash run through it should do what the files record of bash. NEMOSH_OILS_OUT names a
// file to write every case's result to, as JSON.
func TestOilsSpec(t *testing.T) {
	mode := os.Getenv("NEMOSH_OILS")
	switch mode {
	case "":
		t.Skip("set NEMOSH_OILS=report to run the Oils spec suite")
	case "report", "calibrate":
	default:
		t.Fatalf("NEMOSH_OILS=%s: want report or calibrate", mode)
	}
	record := readUpstream(t)
	exclusions, err := oilsspec.ReadExclusions(root)
	if err != nil {
		t.Fatal(err)
	}
	work, err := os.MkdirTemp("", "nemosh-oils-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(work) })
	installed, err := oilsspec.Install(work)
	if err != nil {
		t.Fatal(err)
	}
	subjects := []oilsspec.Subject{{
		Label: "bash", Program: installed.Nemosh, Name: "bash",
		Path: []string{installed.Helpers, filepath.Dir(installed.Nemosh)},
		Env:  passedOn("SystemRoot", "NEMOSH_JOBS"),
	}}
	if mode == "calibrate" {
		subjects = referenceSubjects(t, installed, work)
	}
	all := map[string]map[string][]oilsspec.CaseResult{}
	for i, subject := range subjects {
		results, left := runSuite(t, subject, record, exclusions, filepath.Join(work, fmt.Sprintf("run-%d", i)))
		reportSuite(t, subject, results, left)
		all[subject.Label+" "+filepath.Base(subject.Program)] = results
	}
	if out := os.Getenv("NEMOSH_OILS_OUT"); out != "" {
		data, err := json.MarshalIndent(all, "", " ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(out, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// referenceSubjects are bash, held to what the files record of bash, and busybox, installed
// as ash and held to what they record of ash.
func referenceSubjects(t *testing.T, installed oilsspec.Installed, work string) []oilsspec.Subject {
	t.Helper()
	var subjects []oilsspec.Subject
	if bash := os.Getenv("NEMOSH_OILS_BASH"); bash != "" {
		subjects = append(subjects, oilsspec.Subject{
			Label: "bash", Program: bash, Name: "bash",
			Path: []string{installed.Helpers, filepath.Dir(bash)},
			Env:  passedOn("SystemRoot"),
		})
	}
	if busybox := os.Getenv("NEMOSH_OILS_BUSYBOX"); busybox != "" {
		ash := filepath.Join(work, "busybox", "ash"+filepath.Ext(busybox))
		data, err := os.ReadFile(busybox)
		if err == nil {
			err = os.MkdirAll(filepath.Dir(ash), 0o755)
		}
		if err == nil {
			err = os.WriteFile(ash, data, 0o755)
		}
		if err != nil {
			t.Fatal(err)
		}
		subjects = append(subjects, oilsspec.Subject{
			Label: "ash", Program: ash, Name: "ash",
			Path: []string{installed.Helpers, filepath.Dir(ash)},
			Env:  passedOn("SystemRoot"),
		})
	}
	if len(subjects) == 0 {
		t.Fatal("NEMOSH_OILS=calibrate needs NEMOSH_OILS_BASH or NEMOSH_OILS_BUSYBOX")
	}
	return subjects
}

// runSuite runs every spec file with the subject, the files side by side and the cases of
// each in order, each file with a $REPO_ROOT of its own, since cases write into it. A file
// Oils marks `suite: disabled` is left out, as Oils leaves it out, and so is a case the
// exclusions leave out on this platform; it answers how many each rule left out.
func runSuite(t *testing.T, subject oilsspec.Subject, record oilsspec.Upstream, exclusions oilsspec.Exclusions, work string) (map[string][]oilsspec.CaseResult, map[string]int) {
	t.Helper()
	results := map[string][]oilsspec.CaseResult{}
	left := map[string]int{}
	var mu sync.Mutex
	var group sync.WaitGroup
	slots := make(chan struct{}, runtime.NumCPU())
	for name := range record.Files {
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
		if spec.Metadata["suite"] == "disabled" {
			continue
		}
		file := strings.TrimPrefix(name, "spec/")
		var measured []oilsspec.Case
		for _, c := range spec.Cases {
			if rule, excluded := exclusions.Excluded(file, c, runtime.GOOS); excluded {
				left[rule.Name]++
				continue
			}
			measured = append(measured, c)
		}
		spec.Cases = measured
		group.Add(1)
		go func() {
			defer group.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			dir := filepath.Join(work, strings.TrimSuffix(file, ".test.sh"))
			if err := record.RepoRoot(root, filepath.Join(dir, "repo")); err != nil {
				t.Errorf("%s: %v", file, err)
				return
			}
			fileResults, err := subject.RunSpec(context.Background(), spec, filepath.Join(dir, "cases"), filepath.Join(dir, "repo"))
			if err != nil {
				t.Errorf("%s: %v", file, err)
				return
			}
			mu.Lock()
			results[file] = fileResults
			mu.Unlock()
		}()
	}
	group.Wait()
	return results, left
}

// reportSuite says, for the whole suite and file by file, how many cases the subject's
// runs matched what the files record of bash and of ash, and what was left out.
func reportSuite(t *testing.T, subject oilsspec.Subject, results map[string][]oilsspec.CaseResult, left map[string]int) {
	t.Helper()
	files := make([]string, 0, len(results))
	for file := range results {
		files = append(files, file)
	}
	sort.Strings(files)
	var total, bash, ash, timedOut int
	var lines []string
	for _, file := range files {
		var fileBash, fileAsh int
		for _, result := range results[file] {
			if result.Bash.Matched() {
				fileBash++
			}
			if result.Ash.Matched() {
				fileAsh++
			}
			if result.Bash == oilsspec.Timeout {
				timedOut++
			}
		}
		total += len(results[file])
		bash += fileBash
		ash += fileAsh
		lines = append(lines, fmt.Sprintf("%-32s %4d %4d %4d", file, len(results[file]), fileBash, fileAsh))
	}
	t.Logf("%s (%s, held to %s): %d cases in %d files; %d do what bash does, %d what ash does; %d ran out of time",
		filepath.Base(subject.Program), subject.Name, subject.Label, total, len(files), bash, ash, timedOut)
	t.Logf("%-32s %4s %4s %4s\n%s", "file", "all", "bash", "ash", strings.Join(lines, "\n"))
	rules := make([]string, 0, len(left))
	for rule, count := range left {
		rules = append(rules, fmt.Sprintf("%s %d", rule, count))
	}
	sort.Strings(rules)
	t.Logf("left out on %s, as cases this harness cannot measure here: %s", runtime.GOOS, strings.Join(rules, ", "))
}

// passedOn is the variables of this process's environment that are named and set, as
// NAME=value.
func passedOn(names ...string) []string {
	var env []string
	for _, name := range names {
		if value, set := os.LookupEnv(name); set {
			env = append(env, name+"="+value)
		}
	}
	return env
}
