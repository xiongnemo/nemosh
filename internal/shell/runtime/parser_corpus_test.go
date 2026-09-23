package runtime_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// **A corpus of parser edge cases, with both references as the authority.**
//
// The parser had one-off tests for the constructs somebody had happened to break, and a
// one-liner a person wrote by hand -- `a() { case a in a) echo bingo;; *) echo hmm;; esac; }`
// -- turned out to be two separate faults. That is the sort of thing a corpus catches and a
// list of past bugs does not.
//
// **Written here, not borrowed.** busybox, bash and dash all ship test suites and all three
// are GPL; copying their cases into this tree would put this file under a licence it does
// not have. So these are written from the POSIX 2.9 grammar and from the shapes a script
// really contains, and the *answers* come from running both references -- which is measuring
// behaviour, not copying text. Every expectation in parser_corpus.json is one that busybox
// ash v1.38.0 and bash agreed on, and the cases where they disagreed on the status of a
// rejection are recorded as "must fail" rather than pinned to either.
//
// The data lives in testdata so a case can be added without touching Go, and the generator
// that measured it is not in the tree: the references are, so it can be re-measured. When a
// gap below is fixed, its case moves from parser_gaps.json into parser_corpus.json.

type parserCase struct {
	Script       string `json:"script"`
	Stdout       string `json:"stdout"`
	Status       int    `json:"status"`
	MustFail     bool   `json:"mustFail"`
	Nemosh       string `json:"nemosh"`
	NemoshStatus int    `json:"nemoshStatus"`
}

func loadParserCases(t *testing.T, name string) []parserCase {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var cases []parserCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	if len(cases) == 0 {
		t.Fatalf("%s is empty, so this proves nothing", name)
	}
	return cases
}

// runScriptCapturing runs one case the way `nemosh -c` does, which is what the corpus was
// measured against: RunScript and then CloseBatch, so an EXIT trap fires.
func runScriptCapturing(script string) (string, int) {
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: new(bytes.Buffer)})
	status := rt.RunScript(context.Background(), script)
	rt.CloseBatch(status)
	return stdout.String(), status
}

// caseName makes a readable subtest name out of a script.
func caseName(script string) string {
	name := strings.Join(strings.Fields(strings.ReplaceAll(script, "\n", " ~ ")), " ")
	if len(name) > 70 {
		name = name[:70]
	}
	return name
}

func TestParserCorpus(t *testing.T) {
	for _, testcase := range loadParserCases(t, "parser_corpus.json") {
		t.Run(caseName(testcase.Script), func(t *testing.T) {
			stdout, status := runScriptCapturing(testcase.Script)
			if stdout != testcase.Stdout {
				t.Errorf("stdout = %q, want %q", stdout, testcase.Stdout)
			}
			if testcase.MustFail {
				// The references chose different statuses for this rejection, so what is
				// being asserted is that it *is* rejected.
				if status == 0 {
					t.Errorf("status = 0, want a rejection: both references refuse this")
				}
				return
			}
			if status != testcase.Status {
				t.Errorf("status = %d, want %d", status, testcase.Status)
			}
		})
	}
}

// TestParserGaps is the other half of the corpus: cases both references run and this
// parser does not, each skipped with what it should answer.
//
// Skipped rather than deleted, and skipped rather than asserted as-is. Deleting them would
// lose the measurement; asserting the wrong answer would pin the bug in place. A skip says
// what is missing, and turns into a passing test the moment it is fixed -- at which point
// the case moves into parser_corpus.json and stops being skipped.
//
// What is in here, at the time of writing:
//
//   - **A compound command as the condition of `if`, `while` or `until`.** POSIX 2.9.4 has
//     the condition as a compound_list, so `if { true; }; then`, `if case ... esac; then`,
//     `if for ...; done; then`, and the `if`/`while` forms all belong there. A subshell
//     condition already works, as do a pipeline and a negation, so it is the keyword
//     compounds and the brace group that are missing. Nine entries.
//   - **`esac` as a pattern.** `case a in esac) ...` is rejected by both references and
//     accepted here. Being too permissive about a word nobody writes on purpose, which is
//     why it is last.
//   - **`$(())`**, the empty arithmetic expression, which is zero in both references.
func TestParserGaps(t *testing.T) {
	gaps := loadParserCases(t, "parser_gaps.json")
	for _, testcase := range gaps {
		t.Run(caseName(testcase.Script), func(t *testing.T) {
			stdout, status := runScriptCapturing(testcase.Script)
			matches := stdout == testcase.Stdout
			if testcase.MustFail {
				matches = matches && status != 0
			} else {
				matches = matches && status == testcase.Status
			}
			if matches {
				t.Fatalf("this gap is closed: the references' answer %q/%d is now what this "+
					"parser gives, so move the case into parser_corpus.json",
					testcase.Stdout, testcase.Status)
			}
			t.Skipf("known gap: references answer %q with status %d; this parser answers %q "+
				"with status %d", testcase.Stdout, testcase.Status, stdout, status)
		})
	}
}
