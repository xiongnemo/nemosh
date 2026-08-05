package runtime_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestCommandSubstitution_doesNotMutateParentVariables(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "name=parent\necho $(name=child)\necho $name\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "\nparent\n" {
		t.Fatalf("expected parent variable output %q, got %q", "\nparent\n", got)
	}
}

func TestCommandSubstitution_doesNotMutateParentPositionalParameters(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "set -- first second\necho $(shift)\necho $1 $2\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "\nfirst second\n" {
		t.Fatalf("expected parent positional output %q, got %q", "\nfirst second\n", got)
	}
}

func TestCommandSubstitution_doesNotMutateParentOptions(t *testing.T) {
	// Given
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})
	if status := rt.RunScript(context.Background(), "echo $(set -o pipefail)\n"); status != 0 {
		t.Fatalf("expected substitution setup status 0, got %d", status)
	}

	// When
	status := rt.RunScript(context.Background(), "false | true\n")

	// Then
	if status != 0 {
		t.Fatalf("expected parent pipefail to remain disabled, got status %d", status)
	}
}

func TestCommandSubstitution_doesNotMutateParentReadonlyNames(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})
	if status := rt.RunScript(context.Background(), "echo $(readonly child)\n"); status != 0 {
		t.Fatalf("expected substitution setup status 0, got %d", status)
	}

	// When
	status := rt.RunScript(context.Background(), "child=parent\necho $child\n")

	// Then
	if status != 0 {
		t.Fatalf("expected parent assignment status 0, got %d", status)
	}
	if got := stdout.String(); got != "\nparent\n" {
		t.Fatalf("expected parent readonly isolation output %q, got %q", "\nparent\n", got)
	}
}

func TestCommandSubstitution_resetsChildTrapsAndPreservesParentTraps(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "trap 'echo parent' EXIT\necho $(echo child)\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "child\nparent\n" {
		t.Fatalf("expected trap policy output %q, got %q", "child\nparent\n", got)
	}
}

func TestCommandSubstitution_doesNotMutateParentUmask(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "echo $(umask 077)\numask\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "\n0022\n" {
		t.Fatalf("expected parent umask output %q, got %q", "\n0022\n", got)
	}
}
