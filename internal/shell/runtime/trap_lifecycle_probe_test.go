package runtime

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestRuntime_status130WithoutInterruptDoesNotRunINTTrap(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	registry := applets.NewRegistry(interruptApplet{name: "status130", run: func(context.Context, io.Writer) error {
		return applets.ExitStatus(130)
	}})
	rt := New(registry, Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "trap 'echo interrupted' INT\nstatus130\n")

	// Then
	if status != 130 || stdout.String() != "" {
		t.Fatalf("status = %d, stdout = %q, want status 130 without INT trap output", status, stdout.String())
	}
}

func TestRuntime_interactiveINTTrapRemainsInstalledAcrossInterrupts(t *testing.T) {
	// Given
	started := make(chan struct{}, 2)
	registry := applets.NewRegistry(interruptApplet{name: "interruptible", run: func(ctx context.Context, _ io.Writer) error {
		started <- struct{}{}
		<-ctx.Done()
		return ctx.Err()
	}})
	var stdout bytes.Buffer
	rt := New(registry, Streams{Stdout: &stdout})
	trapScript, err := ParseScript("trap 'echo interrupted' INT\n")
	if err != nil {
		t.Fatalf("ParseScript(trap) error = %v", err)
	}
	commandScript, err := ParseScript("interruptible\n")
	if err != nil {
		t.Fatalf("ParseScript(command) error = %v", err)
	}
	rt.RunInteractive(context.Background(), trapScript)

	// When
	for range 2 {
		ctx, cancel := InterruptContext(context.Background())
		done := make(chan InteractiveResult, 1)
		go func() { done <- rt.RunInteractive(ctx, commandScript) }()
		<-started
		cancel()
		if result := <-done; result.Status != 130 || result.Exited {
			t.Fatalf("RunInteractive() = %+v, want status 130 without exit", result)
		}
	}

	// Then
	if got := stdout.String(); got != "interrupted\ninterrupted\n" {
		t.Fatalf("INT trap output = %q, want one execution per interrupt", got)
	}
}

func TestRuntime_INTTrapReplacementAndClearPersistAcrossInterrupts(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout})
	install, _ := ParseScript("trap 'echo first\ntrap \"echo second\" INT' INT\n")
	clear, _ := ParseScript("trap '' INT\n")
	rt.RunInteractive(context.Background(), install)

	// When
	rt.runInterruptTrap(context.Background(), 130)
	rt.runInterruptTrap(context.Background(), 130)
	rt.RunInteractive(context.Background(), clear)
	rt.runInterruptTrap(context.Background(), 130)

	// Then
	if stdout.String() != "first\nsecond\n" || rt.traps[trapINT] != "" {
		t.Fatalf("stdout = %q, INT trap = %q, want replacement respected then cleared", stdout.String(), rt.traps[trapINT])
	}
}

func TestRuntime_batchExecSuppressesEXITTrapAfterClose(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "trap 'echo trapped' EXIT\nexec true\n")
	rt.CloseBatch(status)

	// Then
	if status != 0 || stdout.String() != "" {
		t.Fatalf("status = %d, stdout = %q, want exec to suppress EXIT trap through Close", status, stdout.String())
	}
}

func TestRuntime_nestedExecStopsOuterScriptAndSuppressesEXIT(t *testing.T) {
	for _, script := range []string{
		"trap 'echo trapped' EXIT\neval exec true\necho after\n",
		"trap 'echo trapped' EXIT\nf() { exec true; }\nf\necho after\n",
	} {
		// Given
		var stdout bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout})

		// When
		status := rt.RunScript(context.Background(), script)
		rt.CloseBatch(status)

		// Then
		if status != 0 || stdout.String() != "" {
			t.Fatalf("script = %q, status = %d, stdout = %q, want nested exec replacement", script, status, stdout.String())
		}
	}
}

func TestRuntime_interactiveExecSuppressesEXITTrapAfterRepeatedClose(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout})
	trapScript, err := ParseScript("trap 'echo trapped' EXIT\n")
	if err != nil {
		t.Fatalf("ParseScript(trap) error = %v", err)
	}
	execScript, err := ParseScript("exec true\n")
	if err != nil {
		t.Fatalf("ParseScript(exec) error = %v", err)
	}
	rt.RunInteractive(context.Background(), trapScript)

	// When
	result := rt.RunInteractive(context.Background(), execScript)
	firstClose := rt.CloseInteractive(context.Background())
	secondClose := rt.CloseInteractive(context.Background())

	// Then
	if result != (InteractiveResult{Status: 0, Exited: true}) || firstClose != 0 || secondClose != 0 || stdout.String() != "" {
		t.Fatalf("result = %+v, close statuses = (%d, %d), stdout = %q, want exec suppression through repeated close", result, firstClose, secondClose, stdout.String())
	}
}
