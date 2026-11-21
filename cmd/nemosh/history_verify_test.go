package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// With `shopt -s histverify` a line that history expansion changed is not run: it is given back
// to edit, as bash's readline gives it back, and not echoed, since it is there on the line to be
// read. A line expansion did not change runs as always. bash does this only where readline reads
// the line, which is the edited loop; the plain loop, a piped session, runs it, as bash's does.
func TestApplyHistoryExpansion_verifyGivesTheLineBackToEdit(t *testing.T) {
	var stderr bytes.Buffer
	line, outcome := applyHistoryExpansionVerifying(fakeHistory{"echo hi"}, &historyExpander{}, &stderr, "!! there", true)
	if outcome != historyVerify || line != "echo hi there" || stderr.Len() != 0 {
		t.Errorf("got %q, outcome %v, stderr %q; want the expanded line given back, unsaid", line, outcome, stderr.String())
	}
	line, outcome = applyHistoryExpansionVerifying(fakeHistory{"echo hi"}, &historyExpander{}, &stderr, "echo plain", true)
	if outcome != historyRun || line != "echo plain" {
		t.Errorf("unchanged: got %q, outcome %v; want it run", line, outcome)
	}
}

// The editor begins a line with the text it is given to start with, the cursor after it, so the
// keys typed next go on from there; the next line begins empty again.
func TestLineEditor_beginsWithTheTextGivenBack(t *testing.T) {
	editor := newLineEditor(strings.NewReader(" there\rnext\r"), newScreenModel(t, 120), t.TempDir())
	editor.start = "echo hi"
	line, err := editor.readLine(context.Background(), "$ ")
	if err != nil || line != "echo hi there" {
		t.Fatalf("got %q, err %v; want the text given back and what was typed after it", line, err)
	}
	if line, err = editor.readLine(context.Background(), "$ "); err != nil || line != "next" {
		t.Errorf("next line: got %q, err %v; want it to begin empty", line, err)
	}
}
