package runtime

import (
	"bytes"
	"context"
	"slices"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// `complete -I` is the specification for a command's first word, which bash runs when Tab is
// pressed there: its function is called with _InitialWorD_, the word, and the word again for
// the word before it, as bash 5.3 calls it, measured by driving Git for Windows' bash at a
// console. It was recorded and never asked, so Tab in a command's name completed as though
// there were none. A word after the name is its command's, and -I has nothing to do with it.
func TestCompleteLine_runsTheInitialWordSpecification(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
	rt.RunScript(context.Background(), `f() { seen="$1|$2|$3|${COMP_WORDS[*]}|$COMP_CWORD|$COMP_LINE|$COMP_POINT"; COMPREPLY=(xxone xxtwo); }`+"\n"+`complete -I -F f`+"\n")
	completion, ok := rt.CompleteLine(context.Background(), "ec", 2)
	if !ok || !slices.Equal(completion.Candidates, []string{"xxone", "xxtwo"}) || completion.Start != 0 {
		t.Fatalf("got %+v, %v; want -I's two candidates for the word at 0", completion, ok)
	}
	if seen, _ := rt.LookupVariable("seen"); seen != "_InitialWorD_|ec|ec|ec|0|ec|2" {
		t.Errorf("-I's function saw %q, want bash's _InitialWorD_|ec|ec|ec|0|ec|2", seen)
	}
	if completion, ok := rt.CompleteLine(context.Background(), "echo a; ls -", 12); ok {
		t.Errorf("an operand of ls: got %+v; want no specification run", completion)
	}
	if completion, ok := rt.CompleteLine(context.Background(), "echo a; l", 9); !ok || !slices.Equal(completion.Candidates, []string{"xxone", "xxtwo"}) || completion.Start != 8 {
		t.Errorf("a second command's name: got %+v, %v; want -I's candidates at 8", completion, ok)
	}
}
