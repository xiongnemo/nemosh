package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A prompt is expanded as the inside of double quotes: a quote of its own is text, and one in a
// command substitution, an arithmetic expansion or a ${...} is that expansion's, as inside
// double quotes. busybox and bash draw PS1='[$(echo "hi")|${y:-"a b"}|"q"|$(echo ")")]> ' as
// [hi|a b|"q"|)]> , and bash's PS0 and ${x@P} make the same of it, measured 2026-11-21. Every
// quote was escaped for the double quotes the text is read in, the substitution's too, so it
// saw \"hi\" and printed the quotes, and `$(echo ")")` came apart.
func TestPrompt_aQuoteInsideAnExpansionIsTheExpansions(t *testing.T) {
	const text = `[$(echo "hi")|${y:-"a b"}|"q"|$(echo ")")|$((1+$(echo 1)))]> `
	const want = `[hi|a b|"q"|)|2]> `
	ctx := context.Background()
	var stdout bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: new(bytes.Buffer)})

	if got := rt.Prompt(ctx, text, 0); got != want {
		t.Errorf("PS1 is drawn %q, want %q", got, want)
	}
	if status := rt.RunScript(ctx, "PS0='"+text+"'"); status != 0 {
		t.Fatalf("setting PS0 exited %d", status)
	}
	if got := rt.CommandPrompt(ctx, 0); got != want {
		t.Errorf("PS0 is %q, want %q", got, want)
	}
	if status := rt.RunScript(ctx, `x='`+text+`'; echo "${x@P}"`); status != 0 || stdout.String() != want+"\n" {
		t.Errorf("${x@P} printed %q, status %d; want %q", stdout.String(), status, want)
	}
}
