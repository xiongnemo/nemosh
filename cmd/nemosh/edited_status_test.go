package main

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// **`$?` at the prompt is the status of the command before it.**
//
// Reported from a real terminal, and as plain as a bug gets:
//
//	$ false
//	# ... C:1 ...          <- the prompt knew
//	$ echo $?
//	0                      <- and this did not
//
// runEditedLine took the Runtime **by value**. Three of its methods have pointer
// receivers, and they are exactly the ones that carry interactive state across commands --
// RunInteractive, ReportInteractiveParseError and CloseInteractive. Calling RunInteractive
// on a copy runs the command correctly and then throws the status away, so the next command
// saw whatever the session started with: zero, every time.
//
// The prompt was right by coincidence. The session keeps its own lastStatus for drawing,
// which is fed by runEditedLine's return value and never asked the runtime -- so the two
// disagreed and only the one nobody could see was wrong.
//
// Everything else survived the copy because it is behind a pointer already: variables,
// functions, aliases, the job scope. `interactive` is a plain struct field, which is why
// this was the one thing that did not.
//
// The cooked loop in session.go was always right: it calls rt.RunInteractive on its own
// variable, so the compiler takes its address. Only the edited path -- the one a person at
// a terminal uses, and the one no test drove end to end -- went through a parameter.
func TestRunEditedLine_carriesTheStatusToTheNextCommand(t *testing.T) {
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: io.Discard})
	shell := command{stdout: &stdout, stderr: io.Discard, registry: applets.DefaultRegistry}
	controller := &interruptController{}
	ctx := context.Background()

	run := func(text string) int {
		t.Helper()
		script, err := runtime.ParseScript(text)
		if err != nil {
			t.Fatalf("parse %q: %v", text, err)
		}
		status, _, runErr := shell.runEditedLine(ctx, &rt, controller, script, 0)
		if runErr != nil {
			t.Fatalf("run %q: %v", text, runErr)
		}
		return status
	}

	if status := run("false\n"); status != 1 {
		t.Fatalf("`false` reported %d, want 1", status)
	}
	stdout.Reset()
	run("echo $?\n")
	if got := stdout.String(); got != "1\n" {
		t.Errorf("`echo $?` after `false` printed %q, want %q -- the status did not survive "+
			"into the next command, which is what a Runtime passed by value does to it", got, "1\n")
	}

	// And that it keeps tracking rather than sticking at one value.
	run("true\n")
	stdout.Reset()
	run("echo $?\n")
	if got := stdout.String(); got != "0\n" {
		t.Errorf("`echo $?` after `true` printed %q, want %q", got, "0\n")
	}
}

// The edited loop writes PS0 as the plain one does: before the command, with $? the status of
// the one before, which the loop hands it.
func TestRunEditedLine_writesPS0BeforeTheCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})
	shell := command{stdout: &stdout, stderr: &stderr, registry: applets.DefaultRegistry}
	controller := &interruptController{}
	for _, step := range []struct {
		text       string
		lastStatus int
	}{{"PS0='<$?>'\n", 0}, {"echo ran\n", 7}} {
		script, err := runtime.ParseScript(step.text)
		if err != nil {
			t.Fatalf("parse %q: %v", step.text, err)
		}
		if _, _, err := shell.runEditedLine(context.Background(), &rt, controller, script, step.lastStatus); err != nil {
			t.Fatalf("run %q: %v", step.text, err)
		}
	}
	if stdout.String() != "ran\n" || stderr.String() != "<7>" {
		t.Fatalf("stdout %q, stderr %q, want %q and %q", stdout.String(), stderr.String(), "ran\n", "<7>")
	}
}
