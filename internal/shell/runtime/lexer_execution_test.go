package runtime_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_preservesQuotedAndEscapedListAndPipeOperatorsAsArguments(t *testing.T) {
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	status := rt.RunScript(context.Background(), `echo "&&" \| '||'`+"\n")

	if status != 0 {
		t.Fatalf("status: %d", status)
	}
	if got, want := stdout.String(), "&& | ||\n"; got != want {
		t.Fatalf("stdout: got %q want %q", got, want)
	}
}

func TestRuntime_doesNotTreatExpansionOutputAsSyntax(t *testing.T) {
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	status := rt.RunScript(context.Background(), "operator='|'\necho $operator tail\n")

	if status != 0 {
		t.Fatalf("status: %d", status)
	}
	if got, want := stdout.String(), "| tail\n"; got != want {
		t.Fatalf("stdout: got %q want %q", got, want)
	}
}

func TestRuntime_keepsOperatorsInsideCommandSubstitutionOutOfOuterSyntax(t *testing.T) {
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	status := rt.RunScript(context.Background(), "echo $(echo inner '|' value)\n")

	if status != 0 {
		t.Fatalf("status: %d", status)
	}
	if got, want := stdout.String(), "inner | value\n"; got != want {
		t.Fatalf("stdout: got %q want %q", got, want)
	}
}

func TestRuntime_preservesCompoundExecutionWithTokenScanner(t *testing.T) {
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	status := rt.RunScript(context.Background(), "for value in 'a|b' 'c&&d'\ndo\necho $value\ndone\ncase x in\nx)\necho case-ok\n;;\nesac\n")

	if status != 0 {
		t.Fatalf("status: %d", status)
	}
	if got, want := stdout.String(), "a|b\nc&&d\ncase-ok\n"; got != want {
		t.Fatalf("stdout: got %q want %q", got, want)
	}
}
