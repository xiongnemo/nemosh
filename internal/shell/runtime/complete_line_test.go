package runtime_test

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// completingShell is a session's runtime that has run script, for CompleteLine to ask.
func completingShell(t *testing.T, script string) runtime.Runtime {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})
	rt.SetInvocationMode("is")
	if status := rt.RunScript(context.Background(), script); status != 0 || stderr.Len() != 0 {
		t.Fatalf("setting up: status %d, stderr %q", status, stderr.String())
	}
	return rt
}

// completeAt asks rt what a Tab would answer with the cursor at the `|` in line.
func completeAt(rt runtime.Runtime, line string) (runtime.ProgrammableCompletion, bool) {
	point := strings.IndexByte(line, '|')
	return rt.CompleteLine(context.Background(), line[:point]+line[point+1:], point)
}

// A function's specification is called as bash calls it: the command, the word and the word
// before it, and the line in COMP_WORDS, COMP_CWORD, COMP_LINE and COMP_POINT, split at
// COMP_WORDBREAKS -- so `--opt=va` is three words and the word is `va` -- and its COMPREPLY is
// the answer, which replaces the text after the `=`. Each value is what bash 5.3's
// bashline.c and pcomplete.c make of this line.
func TestCompleteLine_callsTheFunctionWithTheLineAsBashSaysIt(t *testing.T) {
	rt := completingShell(t, `f() {
  COMPREPLY=("1=$1" "2=$2" "3=$3" "words=${COMP_WORDS[*]}" "cword=$COMP_CWORD" "line=$COMP_LINE" "point=$COMP_POINT" "type=$COMP_TYPE")
}
complete -F f cmd
`)
	answer, found := completeAt(rt, "echo x; X=1 cmd sub --opt=va| rest")
	want := []string{"1=cmd", "2=va", "3==", "words=cmd sub --opt = va rest", "cword=4", "line=cmd sub --opt=va rest", "point=16", "type=9"}
	if !found || !slices.Equal(answer.Candidates, want) || answer.Start != 26 {
		t.Errorf("got %v, %q from %d; want %q from 26", found, answer.Candidates, answer.Start, want)
	}
	// A cursor after a blank is in a word of its own, empty.
	answer, _ = completeAt(rt, "cmd a |")
	if want := []string{"1=cmd", "2=", "3=a", "words=cmd a ", "cword=2", "line=cmd a ", "point=6", "type=9"}; !slices.Equal(answer.Candidates, want) {
		t.Errorf("after a blank: got %q, want %q", answer.Candidates, want)
	}
}

// The other ways a specification answers, and when there is none to ask: a word list filtered
// by the word, the -D specification for a command with none, a function that loads one and
// answers 124, compopt changing the running completion's options and not the specification's,
// and nothing for a command name being typed or a command nothing was said for.
func TestCompleteLine_answersAsTheSpecificationSays(t *testing.T) {
	rt := completingShell(t, `complete -W 'alpha beta gamma' w
loader() { complete -W "x1 x2" "$1"; return 124; }
complete -D -F loader
g() { compopt -o nospace; COMPREPLY=(z); }
complete -o filenames -F g gc
`)
	if answer, _ := completeAt(rt, "w b|"); !slices.Equal(answer.Candidates, []string{"beta"}) || answer.Start != 2 {
		t.Errorf("-W: got %q from %d", answer.Candidates, answer.Start)
	}
	if answer, found := completeAt(rt, "newcmd x|"); !found || !slices.Equal(answer.Candidates, []string{"x1", "x2"}) {
		t.Errorf("-D loading one: got %v, %q", found, answer.Candidates)
	}
	answer, _ := completeAt(rt, "gc |")
	if !slices.Equal(answer.Candidates, []string{"z"}) || !answer.Options["nospace"] || !answer.Options["filenames"] {
		t.Errorf("compopt: got %q, %v", answer.Candidates, answer.Options)
	}
	if again, _ := completeAt(rt, "gc |"); !again.Options["nospace"] || !again.Options["filenames"] {
		t.Errorf("compopt again: got %v", again.Options)
	}
	rt = completingShell(t, "complete -W 'a b' w\n")
	for _, line := range []string{"w|", "ww|", "echo a; w|", "other |"} {
		if answer, found := completeAt(rt, line); found {
			t.Errorf("%q: got %q, want nothing to ask", line, answer.Candidates)
		}
	}
}
