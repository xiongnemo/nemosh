package runtime

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestParseScript_rejectsDeferredSyntax(t *testing.T) {
	tests := []string{
		"name() echo value\n",
		"function name echo value\n",
	}
	for _, source := range tests {
		t.Run(source, func(t *testing.T) {
			// When
			_, err := ParseScript(source)

			// Then
			if err == nil || errors.Is(err, ErrIncompleteScript) {
				t.Fatalf("ParseScript() error = %v, want complete unsupported-syntax error", err)
			}
		})
	}
}

func TestRuntime_RunScript_hasNoPrefixEffects_whenDeferredSyntaxFollowsPrefix(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	runtime := New(applets.DefaultRegistry, Streams{Stdout: &stdout})

	// When
	status := runtime.RunScript(context.Background(), "echo prefix\nfunction deferred echo value\n")

	// Then
	if status != 2 {
		t.Fatalf("RunScript() status = %d, want 2", status)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestParseScript_rejectsInputBeyondLimit(t *testing.T) {
	// Given
	source := "echo " + strings.Repeat("x", maxParseInputBytes)

	// When
	_, err := ParseScript(source)

	// Then
	if !errors.Is(err, errParseLimit) {
		t.Fatalf("ParseScript() error = %v, want errParseLimit", err)
	}
}

func TestParseScript_rejectsCommandSubstitutionBeyondDepthLimit(t *testing.T) {
	// Given
	source := "echo " + strings.Repeat("$(echo ", maxParseDepth+1) + "value" + strings.Repeat(")", maxParseDepth+1)

	// When
	_, err := ParseScript(source)

	// Then
	if !errors.Is(err, errParseLimit) {
		t.Fatalf("ParseScript() error = %v, want errParseLimit", err)
	}
}

func TestParseScript_enforcesSharedCompoundDepthLimit(t *testing.T) {
	build := func(depth int) string {
		return strings.Repeat("if true\nthen\n", depth) + "true\n" + strings.Repeat("fi\n", depth)
	}
	if _, err := ParseScript(build(maxParseDepth)); err != nil {
		t.Fatalf("ParseScript(at limit) error = %v", err)
	}
	if _, err := ParseScript(build(maxParseDepth + 1)); !errors.Is(err, errParseLimit) {
		t.Fatalf("ParseScript(over limit) error = %v, want errParseLimit", err)
	}
}

func TestCompoundSpans_rejectsDepthBeforeAppendingOverLimitFrame(t *testing.T) {
	// Given
	lines := make([]string, 0, maxParseDepth+1)
	for range maxParseDepth + 1 {
		lines = append(lines, "if true")
	}

	// When
	_, err := compoundSpans(lines)

	// Then
	if !errors.Is(err, errParseLimit) {
		t.Fatalf("compoundSpans() error = %v, want errParseLimit", err)
	}
}

func TestCompoundSpans_enforcesExactMixedCompoundDepthBeforeAppend(t *testing.T) {
	build := func(depth int) []string {
		lines := make([]string, 0, depth*3+1)
		closers := make([][]string, 0, depth)
		for index := range depth {
			switch index % 3 {
			case 0:
				lines = append(lines, "if true", "then")
				closers = append(closers, []string{"fi"})
			case 1:
				lines = append(lines, "while true", "do")
				closers = append(closers, []string{"done"})
			case 2:
				lines = append(lines, "case value in", "value)")
				closers = append(closers, []string{";;", "esac"})
			}
		}
		lines = append(lines, "true")
		for index := len(closers) - 1; index >= 0; index-- {
			lines = append(lines, closers[index]...)
		}
		return lines
	}

	// When / Then
	if _, err := compoundSpans(build(maxParseDepth)); err != nil {
		t.Fatalf("compoundSpans(at limit) error = %v", err)
	}
	if _, err := compoundSpans(build(maxParseDepth + 1)); !errors.Is(err, errParseLimit) {
		t.Fatalf("compoundSpans(over limit) error = %v, want errParseLimit", err)
	}
}
