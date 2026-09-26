package oilsspec_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/testutil/oilsspec"
)

// TestOilsSpec runs the vendored Oils spec suite with nemosh and says how far nemosh is
// from bash: of the cases bash itself passes, how many nemosh passes, and how many of the
// rest it does as ash, which is busybox's shell, does. It takes minutes, so it runs only
// when asked:
//
//	NEMOSH_OILS=report go test ./internal/testutil/oilsspec/ -run TestOilsSpec -count=1 -timeout 30m
//
// NEMOSH_OILS=strict also fails for every case that stands otherwise than
// tests/oils/baseline.json says, the case that started passing as much as the one that
// stopped, once it has stood that way in two more runs of its own; one that does not is
// flaky, and is reported. NEMOSH_OILS=update writes the baseline from the run instead.
//
// NEMOSH_OILS=calibrate runs the references instead of nemosh, bash from NEMOSH_OILS_BASH
// and busybox from NEMOSH_OILS_BUSYBOX, and writes what they did to
// tests/oils/calibration.json: bash's passes are the headline's denominator, and the
// harness is faithful when bash run through it does what the files record of bash.
// NEMOSH_OILS_OUT names a file to write every case's result to, as JSON.
func TestOilsSpec(t *testing.T) {
	mode := os.Getenv("NEMOSH_OILS")
	switch mode {
	case "":
		t.Skip("set NEMOSH_OILS=report to run the Oils spec suite")
	case "report", "strict", "update", "calibrate":
	default:
		t.Fatalf("NEMOSH_OILS=%s: want report, strict, update or calibrate", mode)
	}
	exclusions, err := oilsspec.ReadExclusions(root)
	if err != nil {
		t.Fatal(err)
	}
	suite, err := oilsspec.LoadSuite(root, readUpstream(t), exclusions, runtime.GOOS)
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
	if mode == "calibrate" {
		calibrate(t, suite, installed, work)
		return
	}
	nemosh := oilsspec.Subject{
		Label: "bash", Program: installed.Nemosh, Name: "bash",
		Path: []string{installed.Helpers, filepath.Dir(installed.Nemosh)},
		Env:  passedOn("SystemRoot", "NEMOSH_JOBS"),
	}
	results, err := suite.Run(context.Background(), nemosh, work, nil)
	if err != nil {
		t.Fatal(err)
	}
	reportSuite(t, nemosh, suite, results)
	writeResults(t, map[string]map[string][]oilsspec.CaseResult{"nemosh": results})
	calibration, err := oilsspec.ReadCalibration(root)
	if err != nil {
		t.Fatal(err)
	}
	reportHeadline(t, calibration, results)
	switch mode {
	case "strict":
		checkBaseline(t, suite, nemosh, work, results)
	case "update":
		updateBaseline(t, suite, nemosh, work, runtime.GOOS, results)
	}
}

// calibrate runs bash, held to what the files record of bash, and busybox, installed as
// ash and held to what they record of ash, and writes calibration.json from what they did.
func calibrate(t *testing.T, suite oilsspec.Suite, installed oilsspec.Installed, work string) {
	t.Helper()
	bash, busybox := os.Getenv("NEMOSH_OILS_BASH"), os.Getenv("NEMOSH_OILS_BUSYBOX")
	if bash == "" || busybox == "" {
		t.Fatal("NEMOSH_OILS=calibrate needs NEMOSH_OILS_BASH and NEMOSH_OILS_BUSYBOX")
	}
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
	references := map[string]oilsspec.Subject{
		"bash":    {Label: "bash", Program: bash, Name: "bash", Path: []string{installed.Helpers, filepath.Dir(bash)}, Env: passedOn("SystemRoot")},
		"busybox": {Label: "ash", Program: ash, Name: "ash", Path: []string{installed.Helpers, filepath.Dir(ash)}, Env: passedOn("SystemRoot")},
	}
	versions := map[string]string{"bash": firstLine(bash, "--version"), "busybox": firstLine(busybox, "--help")}
	calibration := oilsspec.Calibration{Platform: runtime.GOOS, Measured: time.Now().Format("2006-01-02"), Shells: map[string]oilsspec.Reference{}}
	all := map[string]map[string][]oilsspec.CaseResult{}
	for _, name := range []string{"bash", "busybox"} {
		subject := references[name]
		results, err := suite.Run(context.Background(), subject, work, nil)
		if err != nil {
			t.Fatal(err)
		}
		reportSuite(t, subject, suite, results)
		all[name] = results
		calibration.Shells[name] = oilsspec.Reference{Version: versions[name], Label: subject.Label, Unmatched: oilsspec.Unmatched(subject.Label, results)}
	}
	writeResults(t, all)
	if err := calibration.Write(root); err != nil {
		t.Fatal(err)
	}
}

// reportSuite says, for the whole suite and file by file, how many cases the subject's
// runs matched what the files record of bash and of ash, and what was left out.
func reportSuite(t *testing.T, subject oilsspec.Subject, suite oilsspec.Suite, results map[string][]oilsspec.CaseResult) {
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
	rules := make([]string, 0, len(suite.Left))
	for rule, count := range suite.Left {
		rules = append(rules, fmt.Sprintf("%s %d", rule, count))
	}
	sort.Strings(rules)
	t.Logf("left out on %s, as cases this harness cannot measure here: %s", runtime.GOOS, strings.Join(rules, ", "))
}

// reportHeadline says how far nemosh is from bash: of the measured cases bash passes, as
// calibration.json records them, how many nemosh passes, and how many of the rest it does
// as ash does.
func reportHeadline(t *testing.T, calibration oilsspec.Calibration, results map[string][]oilsspec.CaseResult) {
	t.Helper()
	bash := calibration.Shells["bash"]
	var passedByBash, passes, ashOnly int
	for file, fileResults := range results {
		for _, result := range fileResults {
			if !bash.Passes(file, result.ID) {
				continue
			}
			passedByBash++
			switch {
			case result.Bash.Matched():
				passes++
			case result.Ash.Matched():
				ashOnly++
			}
		}
	}
	if passedByBash == 0 {
		t.Fatal("calibration.json has bash pass no case this run measured")
	}
	t.Logf("nemosh passes %d of the %d cases bash passes, %.1f%%, and does %d of the rest as ash does",
		passes, passedByBash, 100*float64(passes)/float64(passedByBash), ashOnly)
	if calibration.Platform != runtime.GOOS {
		t.Logf("bash's passes were measured on %s, and this is %s", calibration.Platform, runtime.GOOS)
	}
}

// writeResults writes every case's result to the file NEMOSH_OILS_OUT names, if it names one.
func writeResults(t *testing.T, results map[string]map[string][]oilsspec.CaseResult) {
	t.Helper()
	out := os.Getenv("NEMOSH_OILS_OUT")
	if out == "" {
		return
	}
	data, err := json.MarshalIndent(results, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// firstLine is the first line a program prints, which is how a reference names its version.
func firstLine(program string, args ...string) string {
	out, _ := exec.Command(program, args...).CombinedOutput()
	line, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	return strings.TrimSpace(line)
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
