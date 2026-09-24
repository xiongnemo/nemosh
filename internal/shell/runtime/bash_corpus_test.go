package runtime_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// **The bash-compatibility corpus, each case answered by the reference that governs it.**
//
// The question it keeps answering is "how far is this from bash", but the order of authority
// is the project's own: busybox-w32 first, and bash only for what busybox does not have. So
// every case names its reference. `busybox-w32` means busybox implements the construct and
// its answer is the one to match -- `kill -0`, `$FUNCNAME`, `trap ERR`, what a killed job's
// status is. `bash-5.3` means it is an extension busybox lacks, `printf -v` or `${x@Q}`, and
// bash decides. Where the two disagree and busybox has the construct, busybox wins: `wait`
// on a job whose end `jobs` already reported is 127 there, as POSIX says for a process id the
// shell no longer knows, and 7 in bash.
//
// The answers were measured, not copied, for the reason parser_corpus_test.go gives: both
// references' test suites are GPL. They were measured on busybox-w32 v1.38.0 and on GNU bash
// 5.3.15, and never on macOS's /bin/bash, which is 3.2 and has no associative arrays.
//
// bash_corpus.json is what already agrees, kept so it stays that way. bash_gaps.json is what
// does not yet, each case skipped with the answer it should give -- the same arrangement as
// parser_gaps.json, so a fix fails TestBashGaps until its case moves across.

type bashCase struct {
	Script    string `json:"script"`
	Stdout    string `json:"stdout"`
	Status    int    `json:"status"`
	Reference string `json:"reference"`
	// Jobs is the launcher the case's answer depends on, "process" or "goroutine", and
	// empty for a case that holds under either. The numeric `$!` is the one that needs it:
	// only a job that is a process has a pid to give.
	Jobs string `json:"jobs,omitempty"`
}

// launcherApplies reports whether the case's launcher is the one these tests run under
// (runtime.JobsAreProcesses, from NEMOSH_JOBS).
func (c bashCase) launcherApplies() bool {
	switch c.Jobs {
	case "process":
		return runtime.JobsAreProcesses()
	case "goroutine":
		return !runtime.JobsAreProcesses()
	}
	return true
}

func loadBashCases(t *testing.T, name string) []bashCase {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var cases []bashCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	if len(cases) == 0 {
		t.Fatalf("%s is empty, so this proves nothing", name)
	}
	for _, testcase := range cases {
		// A case with no reference has an answer nobody can re-measure.
		if testcase.Reference != "busybox-w32" && testcase.Reference != "bash-5.3" {
			t.Fatalf("%q names reference %q; want busybox-w32 or bash-5.3", testcase.Script, testcase.Reference)
		}
		if testcase.Jobs != "" && testcase.Jobs != "process" && testcase.Jobs != "goroutine" {
			t.Fatalf("%q names launcher %q; want process, goroutine, or none", testcase.Script, testcase.Jobs)
		}
	}
	return cases
}

func TestBashCorpus(t *testing.T) {
	for _, testcase := range loadBashCases(t, "bash_corpus.json") {
		t.Run(caseName(testcase.Script), func(t *testing.T) {
			if !testcase.launcherApplies() {
				t.Skipf("holds only when jobs are %ss", testcase.Jobs)
			}
			stdout, status := runScriptCapturing(testcase.Script)
			if stdout != testcase.Stdout || status != testcase.Status {
				t.Errorf("got %q/%d, want %q/%d, as %s answers", stdout, status,
					testcase.Stdout, testcase.Status, testcase.Reference)
			}
		})
	}
}

func TestBashGaps(t *testing.T) {
	for _, testcase := range loadBashCases(t, "bash_gaps.json") {
		t.Run(caseName(testcase.Script), func(t *testing.T) {
			if !testcase.launcherApplies() {
				t.Skipf("a gap only when jobs are %ss", testcase.Jobs)
			}
			stdout, status := runScriptCapturing(testcase.Script)
			if stdout == testcase.Stdout && status == testcase.Status {
				t.Fatalf("this gap is closed: %s's answer %q/%d is now what this shell gives, "+
					"so move the case into bash_corpus.json", testcase.Reference, testcase.Stdout, testcase.Status)
			}
			t.Skipf("known gap: %s answers %q with status %d; this shell answers %q with status %d",
				testcase.Reference, testcase.Stdout, testcase.Status, stdout, status)
		})
	}
}
