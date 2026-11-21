package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// `shopt -u promptvars` is bash's, and draws a prompt with its escapes decoded and nothing
// expanded; busybox has no such option, so it is bash's under every name. bash's ${x@P} of
// '$(echo sub)|a\nb' is "$(echo sub)|a", a newline and "b" with it off, measured 2026-11-21.
// It was recorded and changed nothing: a prompt run through ${PS1@P} -- oh-my-posh's, which
// turns promptvars off around it for this reason -- ran whatever command substitution a
// directory's name held.
func TestPrompt_promptvarsOffExpandsNothing(t *testing.T) {
	const text = `$(echo sub)|a\nb`
	ctx := context.Background()
	var stdout bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: new(bytes.Buffer)})
	if got := rt.Prompt(ctx, text, 0); got != "sub|a\nb" {
		t.Errorf("with promptvars on PS1 is %q", got)
	}
	if status := rt.RunScript(ctx, "shopt -u promptvars\nPS0='"+text+"'\nx='"+text+"'\necho \"${x@P}\"\n"); status != 0 {
		t.Fatalf("setup exited %d", status)
	}
	const want = "$(echo sub)|a\nb"
	if got := rt.Prompt(ctx, text, 0); got != want {
		t.Errorf("with promptvars off PS1 is %q, want %q", got, want)
	}
	if got := rt.CommandPrompt(ctx, 0); got != want {
		t.Errorf("with promptvars off PS0 is %q, want %q", got, want)
	}
	if stdout.String() != want+"\n" {
		t.Errorf("with promptvars off ${x@P} is %q, want %q", stdout.String(), want)
	}
}
