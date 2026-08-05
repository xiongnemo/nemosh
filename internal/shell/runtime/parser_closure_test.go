package runtime

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestParseScript_enforcesAggregateTokenLimitAtExactBoundary(t *testing.T) {
	// Given
	atLimit := strings.Repeat("x&", maxParseTokens/2)
	overLimit := atLimit + "x"

	// When / Then
	if _, err := ParseScript(atLimit); err != nil {
		t.Fatalf("ParseScript(at limit) error = %v", err)
	}
	if _, err := ParseScript(overLimit); !errors.Is(err, errParseLimit) {
		t.Fatalf("ParseScript(over limit) error = %v, want errParseLimit", err)
	}
}

func TestScanShellTokens_stopsAggregateBudgetBeforeOverLimitAppend(t *testing.T) {
	// Given
	budget := &parseBudget{tokens: maxParseTokens - 1}

	// When
	_, err := scanShellTokensWithBudget("x&", budget, 0)

	// Then
	if !errors.Is(err, errParseLimit) || budget.tokens != maxParseTokens {
		t.Fatalf("error = %v, consumed = %d, want limit error at %d", err, budget.tokens, maxParseTokens)
	}
}

func TestMatchingGroupEnd_enforcesMixedDelimiterDepthBeforeAppend(t *testing.T) {
	build := func(depth int) string {
		var open strings.Builder
		closers := make([]string, 0, depth)
		for index := range depth {
			if index%2 == 0 {
				open.WriteString("( ")
				closers = append(closers, " )")
				continue
			}
			open.WriteString("{ ")
			closers = append(closers, "; }")
		}
		var close strings.Builder
		for index := len(closers) - 1; index >= 0; index-- {
			close.WriteString(closers[index])
		}
		return open.String() + `echo "$(printf ') {')" \( '{'` + close.String()
	}

	// When / Then
	if source := build(maxParseDepth); func() bool {
		_, err := matchingGroupEnd(source, 0, source[0])
		return err == nil
	}() == false {
		t.Fatal("matchingGroupEnd(at limit) rejected mixed nesting")
	}
	source := build(maxParseDepth + 1)
	if _, err := matchingGroupEnd(source, 0, source[0]); !errors.Is(err, errParseLimit) {
		t.Fatalf("matchingGroupEnd(over limit) error = %v, want errParseLimit", err)
	}
}

func TestParseScript_rejectsEmptyInteriorAndOrSegmentsAsMalformed(t *testing.T) {
	malformed := []string{
		"a && && b", "a || || b", "a && || b", "a || && b",
		"a & b && && c", "a && && b & c", "a & b || && c",
	}
	for _, source := range malformed {
		t.Run(source, func(t *testing.T) {
			_, err := ParseScript(source)
			if err == nil || errors.Is(err, ErrIncompleteScript) {
				t.Fatalf("ParseScript() error = %v, want malformed syntax", err)
			}
		})
	}
	for _, source := range []string{"a &&", "a ||", "a |"} {
		t.Run("incomplete "+source, func(t *testing.T) {
			_, err := ParseScript(source)
			if !errors.Is(err, ErrIncompleteScript) {
				t.Fatalf("ParseScript() error = %v, want ErrIncompleteScript", err)
			}
		})
	}
}

func TestParseScript_preservesCompoundCloserLogicalOperatorIncompleteness(t *testing.T) {
	tests := []string{
		"if true\nthen\ntrue\nfi &&\n",
		"while false\ndo\ntrue\ndone &&\n",
		"case x in\nx)\ntrue\n;;\nesac &&\n",
		"f() { true; } &&\n",
	}
	for _, source := range tests {
		t.Run(source, func(t *testing.T) {
			// When
			_, err := ParseScript(source)

			// Then
			if !errors.Is(err, ErrIncompleteScript) {
				t.Fatalf("ParseScript() error = %v, want ErrIncompleteScript", err)
			}
		})
	}
}

func TestTrailingBackground_consumesOnlyActiveStandaloneFinalAmpersand(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		wantLine   string
		background bool
	}{
		{name: "ordinary", line: "echo ready &", wantLine: "echo ready", background: true},
		{name: "compound closer", line: "fi &", wantLine: "fi", background: true},
		{name: "function closer", line: "f() { true; } &", wantLine: "f() { true; }", background: true},
		{name: "and-if", line: "fi &&", wantLine: "fi &&"},
		{name: "quoted", line: `echo "&"`, wantLine: `echo "&"`},
		{name: "escaped", line: `echo \&`, wantLine: `echo \&`},
		{name: "input duplication", line: "cat <&0", wantLine: "cat <&0"},
		{name: "output duplication", line: "cat >&1", wantLine: "cat >&1"},
		{name: "incomplete input duplication", line: "cat <&", wantLine: "cat <&"},
		{name: "incomplete output duplication", line: "cat >&", wantLine: "cat >&"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			line, background := trailingBackground(test.line)

			// Then
			if line != test.wantLine || background != test.background {
				t.Fatalf("trailingBackground(%q) = (%q, %v), want (%q, %v)", test.line, line, background, test.wantLine, test.background)
			}
		})
	}
}

func TestRuntime_malformedBackgroundMatrixHasNoPrefixEffects(t *testing.T) {
	malformed := []string{"&", "& echo bad", "echo ok & & echo bad", "echo ok && &", "echo ok | &", "echo ok &; echo bad", "echo ok & echo x && && echo bad"}
	for _, suffix := range malformed {
		t.Run(suffix, func(t *testing.T) {
			dir := t.TempDir()
			var stdout bytes.Buffer
			rt := NewWithState(applets.DefaultRegistry, Streams{Stdout: &stdout}, State{Cwd: WorkingDirectory(dir)})
			source := "echo prefix\nf() { echo body; }\necho file > marker\n" + suffix + "\n"

			status := rt.RunScript(context.Background(), source)

			if status != 2 || stdout.Len() != 0 || len(rt.functions) != 0 {
				t.Fatalf("status = %d, stdout = %q, functions = %#v", status, stdout.String(), rt.functions)
			}
			if _, err := os.Stat(filepath.Join(dir, "marker")); !os.IsNotExist(err) {
				t.Fatalf("marker exists or stat failed: %v", err)
			}
		})
	}
}
