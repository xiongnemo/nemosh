package runtime_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_runsBothCommands_whenASemicolonSeparatesThem(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "echo a ; echo b\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "a\nb\n" {
		t.Fatalf("expected %q, got %q", "a\nb\n", got)
	}
}

func TestRuntime_reportsTheLastStatus_whenASequentialListEndsInFailure(t *testing.T) {
	// Given
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{})

	// When
	status := rt.RunScript(context.Background(), "true; false\n")

	// Then
	if status != 1 {
		t.Fatalf("expected status 1, got %d", status)
	}
}

func TestRuntime_runsEveryCommand_whenASequentialListLeadsWithAFailure(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "false; echo reached\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "reached\n" {
		t.Fatalf("expected %q, got %q", "reached\n", got)
	}
}

func TestRuntime_acceptsAOneLineIf_whenSemicolonsSeparateTheKeywords(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "if true; then echo yes; else echo no; fi\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "yes\n" {
		t.Fatalf("expected %q, got %q", "yes\n", got)
	}
}

func TestRuntime_acceptsAOneLineForLoop_whenSemicolonsSeparateTheKeywords(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "for i in 1 2; do echo $i; done\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "1\n2\n" {
		t.Fatalf("expected %q, got %q", "1\n2\n", got)
	}
}

func TestRuntime_acceptsAOneLineWhileLoop_whenSemicolonsSeparateTheKeywords(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "while false; do echo never; done; echo after\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "after\n" {
		t.Fatalf("expected %q, got %q", "after\n", got)
	}
}

func TestRuntime_keepsTheSemicolonAsData_whenItIsQuoted(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "echo 'a;b' \"c;d\" e\\;f\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "a;b c;d e;f\n" {
		t.Fatalf("expected %q, got %q", "a;b c;d e;f\n", got)
	}
}

func TestRuntime_ignoresAnEmptySegment_whenTheLineEndsWithASemicolon(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "echo a;\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "a\n" {
		t.Fatalf("expected %q, got %q", "a\n", got)
	}
}

func TestRuntime_reportsSyntaxError_whenACaseTerminatorAppearsOutsideCase(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "echo a ;; echo b\n")

	// Then
	if status != 2 {
		t.Fatalf("expected status 2, got %d", status)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no output before the parse error, got %q", stdout.String())
	}
}

func TestRuntime_keepsGroupBodiesIntact_whenTheyContainSemicolons(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "{ echo a; echo b; } ; echo c\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "a\nb\nc\n" {
		t.Fatalf("expected %q, got %q", "a\nb\nc\n", got)
	}
}

func TestRuntime_keepsSubstitutionBodiesIntact_whenTheyContainSemicolons(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// The substitution is quoted so this pins the separator alone: unquoted, the
	// two lines would additionally be split into two fields, which is expansion
	// work rather than separator work.
	// When
	status := rt.RunScript(context.Background(), "echo \"$(echo a; echo b)\"\n")

	// Then
	if status != 0 {
		t.Fatalf("expected status 0, got %d", status)
	}
	if got := stdout.String(); got != "a\nb\n" {
		t.Fatalf("expected %q, got %q", "a\nb\n", got)
	}
}
