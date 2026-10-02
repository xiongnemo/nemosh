package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// C-x C-e hands the line being typed to the session rather than submitting it.
func TestLineEditor_ctrlXCtrlEHandsTheLineOn(t *testing.T) {
	line, err, _ := editLine(t, "echo hi\x18\x05")
	if line != "echo hi" || !errors.Is(err, errEditAndExecute) {
		t.Fatalf("C-x C-e gave %q, %v; want the line and errEditAndExecute", line, err)
	}
}

// The editor is VISUAL, else EDITOR, run on a file holding the line; what it leaves is said
// on standard error and answered, and an editor that fails answers nothing, as bash's
// readline has it.
func TestEditInEditor_runsTheEditorOnTheLine(t *testing.T) {
	for _, test := range []struct {
		name, setup, want, said string
		ok                      bool
	}{
		{name: "EDITOR", setup: "unset VISUAL; EDITOR='sed -i s/hi/there/'", want: "echo there", said: "echo there\n", ok: true},
		{name: "VISUAL before EDITOR", setup: "VISUAL='sed -i s/hi/visual/'; EDITOR=false", want: "echo visual", said: "echo visual\n", ok: true},
		{name: "an editor that fails", setup: "unset VISUAL; EDITOR=false", ok: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})
			rt.RunScript(context.Background(), test.setup+"\n")
			c := command{stdin: strings.NewReader(""), stdout: &stdout, stderr: &stderr}

			// When
			got, ok := c.editInEditor(context.Background(), &rt, &interruptController{}, "echo hi", false)

			// Then
			if got != test.want || ok != test.ok {
				t.Fatalf("got %q, %v; want %q, %v; stderr %q", got, ok, test.want, test.ok, stderr.String())
			}
			if test.ok && stderr.String() != test.said {
				t.Fatalf("stderr = %q, want %q", stderr.String(), test.said)
			}
		})
	}
}
