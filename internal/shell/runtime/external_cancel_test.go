package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A program is left to end by itself only when the console's Ctrl-C reached it too and the
// setting says so; any other reason the shell stops it ends it. See external_cancel.go.
func TestLeaveToProgram(t *testing.T) {
	for _, test := range []struct {
		name    string
		cause   error
		setting string
		shares  bool
		want    bool
	}{
		{name: "Ctrl-C at the prompt", cause: consoleInterrupt{prompt: true}, shares: true, want: true},
		{name: "Ctrl-C in a script", cause: consoleInterrupt{}, shares: true},
		{name: "program, in a script", cause: consoleInterrupt{}, setting: "program", shares: true, want: true},
		{name: "kill, at the prompt", cause: consoleInterrupt{prompt: true}, setting: "kill", shares: true},
		{name: "a program not on the console", cause: consoleInterrupt{prompt: true}, setting: "program"},
		{name: "an interrupt that is not the console's", cause: errShellInterrupt, setting: "program", shares: true},
		{name: "timeout", cause: context.DeadlineExceeded, setting: "program", shares: true},
		{name: "a pipe no one reads", cause: jobSignal(13), setting: "program", shares: true},
		{name: "cancelled", cause: context.Canceled, setting: "program", shares: true},
	} {
		if got := leaveToProgram(test.cause, test.setting, test.shares); got != test.want {
			t.Errorf("%s: left to the program %v, want %v", test.name, got, test.want)
		}
	}
}

// The console's interrupt is the shell's interrupt to everything else that asks: status 130,
// and applets.ErrInterrupt for tee -i.
func TestConsoleInterrupt_isTheShellsInterrupt(t *testing.T) {
	ctx, interrupt, _ := ConsoleInterruptContextWithRelease(context.Background(), true)

	// When
	interrupt()

	// Then
	if !IsShellInterrupt(ctx) || contextStatus(ctx) != 130 || !errors.Is(context.Cause(ctx), applets.ErrInterrupt) {
		t.Fatalf("cause %v: shell interrupt %v, status %d", context.Cause(ctx), IsShellInterrupt(ctx), contextStatus(ctx))
	}
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	if !leaveToProgram(context.Cause(child), "", true) {
		t.Fatalf("a context derived from it lost the cause: %v", context.Cause(child))
	}
}
