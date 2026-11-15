package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// caseScan and caseWords answer what casePatternPosition and insideCase answered, read once
// rather than from the start at every question. The functions as they were written are kept
// below as the oracle, and every text this repository holds is asked at every bracket, in
// order, the way the scans ask -- and at every seventh byte besides, since a question can
// come wherever a caller's scan has got to.

func TestCaseScan_answersAsTheWholePrefixReadingDid(t *testing.T) {
	texts := caseScanCorpus(t)
	asked := 0
	for _, text := range texts {
		cases := caseScan{text: text}
		words := caseWords{text: text}
		for index := 0; index < len(text); index++ {
			if text[index] != '(' && text[index] != ')' && index%7 != 0 {
				continue
			}
			asked++
			if got, want := cases.patternAt(index), casePatternPositionAsWritten(text[:index]); got != want {
				t.Fatalf("patternAt(%d) = %v, the prefix read whole says %v, in:\n%s", index, got, want, text)
			}
			if got, want := words.openAt(index), insideCaseAsWritten(text[:index]); got != want {
				t.Fatalf("openAt(%d) = %v, the prefix read whole says %v, in:\n%s", index, got, want, text)
			}
		}
	}
	t.Logf("%d texts, %d questions", len(texts), asked)
}

// The prefix may end in the middle of anything the scan looks ahead for, and must be answered
// as the prefix, not as the text it will become.
func TestCaseScan_answersAPrefixThatEndsInsideAConstruct(t *testing.T) {
	for _, text := range []string{
		"case x in (( 1 + 2 )) ) a;; esac",
		"( case $'a)b' in a) x;; esac )",
		"{ case a in a) echo $(case b in b) y;; esac);; esac; }",
		"case a in a) ((i++));; b) echo ${x%)};; esac",
		"case a in a) x;;& b) y;& c) z;; esac",
		"case $'unterminated in a) x;; esac",
		"case a in a) echo $(unterminated",
		"case a in a) ((unterminated",
		"case a in a) x ;",
		"case case in case) x;; esac",
		"( case esac in \"esac\") x;; esac )",
		"case a in a) for c in 1; do :; done;; b) case c in c) :;; esac;; esac",
		"case \\) in \\)) x;; esac",
		"case a in 'a)' | \"b)\") x;; esac",
	} {
		cases := caseScan{text: text}
		words := caseWords{text: text}
		for index := 0; index <= len(text); index++ {
			if got, want := cases.patternAt(index), casePatternPositionAsWritten(text[:index]); got != want {
				t.Fatalf("patternAt(%d) = %v, want %v, in %q", index, got, want, text)
			}
			if got, want := words.openAt(index), insideCaseAsWritten(text[:index]); got != want {
				t.Fatalf("openAt(%d) = %v, want %v, in %q", index, got, want, text)
			}
		}
	}
}

// A scanner building a logical line hands the same scan a longer text each time.
func TestCaseScan_goesOnWithALongerText(t *testing.T) {
	text := "{ case a in a) x ;; b) ( y ) ;; esac; }"
	var cases caseScan
	var words caseWords
	for end := 0; end <= len(text); end++ {
		cases.extend(text[:end])
		words.extend(text[:end])
		if got, want := cases.patternAt(end), casePatternPositionAsWritten(text[:end]); got != want {
			t.Fatalf("patternAt(%d) = %v, want %v", end, got, want)
		}
		if got, want := words.openAt(end), insideCaseAsWritten(text[:end]); got != want {
			t.Fatalf("openAt(%d) = %v, want %v", end, got, want)
		}
	}
	// And a shorter one is a new text.
	cases.extend("case a in")
	if cases.patternAt(len("case a in")) != casePatternPositionAsWritten("case a in") {
		t.Fatal("a shorter text went on from the longer one")
	}
}

// caseScanCorpus is every Oils case, every script of the parser and bash corpora, and the
// adversarial texts above.
func caseScanCorpus(t *testing.T) []string {
	t.Helper()
	var texts []string
	files, err := filepath.Glob(filepath.Join("..", "..", "..", "tests", "oils", "spec", "*.test.sh"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no Oils spec files found (%v)", err)
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		texts = append(texts, strings.Split(string(data), "\n#### ")...)
	}
	for _, name := range []string{"parser_corpus.json", "parser_gaps.json", "bash_corpus.json", "bash_gaps.json"} {
		data, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		var scripts []struct {
			Script string `json:"script"`
		}
		if err := json.Unmarshal(data, &scripts); err != nil || len(scripts) == 0 && !strings.HasSuffix(name, "_gaps.json") {
			t.Fatalf("%s: %d scripts (%v)", name, len(scripts), err)
		}
		for _, script := range scripts {
			texts = append(texts, script.Script)
		}
	}
	return texts
}
