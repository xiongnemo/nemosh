package runtime

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// As nemosh and sh an alias that is syntax is a syntax error, on purpose, and the error is
// followed by a hint that says so and names bash mode; an error no alias explains gets none, and
// bash mode reads such an alias as bash does.
func TestAliases_thatAreSyntaxAreRefusedWithAHint(t *testing.T) {
	const hint = "hint: alias %s is part of a command's syntax, which nemosh does not read on purpose; bash mode does: nemosh bash, or NEMOSH_COMPAT=bash\n"
	for _, test := range []struct{ name, script, alias string }{
		{name: "a script whose structure needs one", script: "alias LEFT='{'\nLEFT echo one; echo two; }\n", alias: "LEFT='{'"},
		{name: "half a loop", script: "alias e_='for i in 1 2 3; do echo '\ne_ $i; done\n", alias: "e_='for i in 1 2 3; do echo '"},
		{name: "eval's text, with one defined", script: "alias LEFT='{'\neval 'LEFT echo one; echo two; }'\n", alias: "LEFT='{'"},
		{name: "one whose text does not parse as it runs", script: "alias e_='for i in 1 2 3; do echo '\ne_ hi\n", alias: "e_='for i in 1 2 3; do echo '"},
		{name: "an error no alias explains", script: "alias ll='echo ll'\nif then\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
			status := rt.RunScript(context.Background(), test.script)
			wantHint := ""
			if test.alias != "" {
				wantHint = strings.Replace(hint, "%s", test.alias, 1)
			}
			if status != 2 || stdout.String() != "" || !strings.HasSuffix(stderr.String(), wantHint) || wantHint == "" && strings.Contains(stderr.String(), "hint:") {
				t.Fatalf("status %d, stdout %q, stderr %q; want 2 and the hint %q", status, stdout.String(), stderr.String(), wantHint)
			}
		})
	}
}

// At a prompt the line that does not parse is the one a hint is about, with the aliases the
// session has defined.
func TestAliases_thatAreSyntaxAreRefusedWithAHintAtAPrompt(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
	script, err := rt.ParseSessionInput("alias LEFT='('\n")
	if err != nil {
		t.Fatal(err)
	}
	rt.RunInteractive(context.Background(), script)
	if _, err = rt.ParseSessionInput("LEFT echo one; echo two )\n"); err == nil || errors.Is(err, ErrIncompleteScript) {
		t.Fatalf("parse: %v, want a syntax error", err)
	}
	rt.ReportInteractiveParseError(err)
	if !strings.Contains(stderr.String(), "hint: alias LEFT='(' is part of a command's syntax") {
		t.Fatalf("stderr %q, want the hint", stderr.String())
	}
}

// bash mode says nothing of the kind: it reads the alias.
func TestAliases_thatAreSyntaxGetNoHintInBashMode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
	rt.EnterBashMode()
	rt.RunScript(context.Background(), "shopt -s expand_aliases\nalias LEFT='{'\nLEFT echo one; echo two; }\nif then\n")
	if stdout.String() != "one\ntwo\n" || strings.Contains(stderr.String(), "hint:") {
		t.Fatalf("stdout %q, stderr %q; want one, two and no hint", stdout.String(), stderr.String())
	}
}
