package runtime_test

import (
	"strings"
	"testing"
)

func TestRuntime_appliesTheParameterOperators(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   string
	}{
		{name: "plain name", script: "x=value\necho [${x}]\n", want: "[value]\n"},
		{name: "length", script: "x=abcd\necho [${#x}]\n", want: "[4]\n"},
		{name: "length counts runes", script: "x=中文\necho [${#x}]\n", want: "[2]\n"},
		{name: "length of positional count", script: "set a b c\necho [${#@}]\n", want: "[3]\n"},

		{name: "default when unset", script: "echo [${nope-fallback}]\n", want: "[fallback]\n"},
		{name: "default kept when set", script: "x=set\necho [${x-fallback}]\n", want: "[set]\n"},
		{name: "default kept when empty", script: "x=\necho [${x-fallback}]\n", want: "[]\n"},
		{name: "colon default when empty", script: "x=\necho [${x:-fallback}]\n", want: "[fallback]\n"},

		{name: "alternate when set", script: "x=set\necho [${x+alt}]\n", want: "[alt]\n"},
		{name: "alternate skipped when unset", script: "echo [${nope+alt}]\n", want: "[]\n"},
		{name: "colon alternate skipped when empty", script: "x=\necho [${x:+alt}]\n", want: "[]\n"},
		{name: "colon alternate when non-empty", script: "x=v\necho [${x:+alt}]\n", want: "[alt]\n"},

		{name: "assign when unset", script: "echo [${nope=assigned}]\necho [$nope]\n", want: "[assigned]\n[assigned]\n"},
		{name: "assign skipped when set", script: "x=kept\necho [${x=assigned}]\n", want: "[kept]\n"},

		{name: "shortest suffix", script: "x=a.b.c\necho [${x%.*}]\n", want: "[a.b]\n"},
		{name: "longest suffix", script: "x=a.b.c\necho [${x%%.*}]\n", want: "[a]\n"},
		{name: "shortest prefix", script: "x=a.b.c\necho [${x#*.}]\n", want: "[b.c]\n"},
		{name: "longest prefix", script: "x=a.b.c\necho [${x##*.}]\n", want: "[c]\n"},
		{name: "suffix that does not match", script: "x=notes\necho [${x%.txt}]\n", want: "[notes]\n"},
		{name: "literal suffix", script: "x=notes.txt\necho [${x%.txt}]\n", want: "[notes]\n"},

		{name: "positional by number", script: "set a b c\necho [${2}]\n", want: "[b]\n"},
		{name: "positional default", script: "set a\necho [${2-none}]\n", want: "[none]\n"},
		{name: "status parameter", script: "false\necho [${?}]\n", want: "[1]\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q, want 0", status, stderr)
			}
			if stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}

func TestRuntime_stopsWithTheGivenMessage_whenAQuestionOperatorFindsNothingSet(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "echo [${nope?is required}]\necho STILL\n")

	// Then
	if status != 2 || stdout != "" {
		t.Fatalf("status = %d, stdout = %q, want 2 and no output", status, stdout)
	}
	if !strings.Contains(stderr, "is required") || !strings.Contains(stderr, "nope") {
		t.Fatalf("stderr = %q, want the name and the message", stderr)
	}
}

func TestRuntime_refusesAnOperatorItDoesNotImplement(t *testing.T) {
	// An unrecognised operator used to expand to its own literal text and exit
	// 0, so an operator this shell did not have silently became data.
	//
	// The example was `${x//a/b}` until replacement was implemented; it is now one
	// of bash's `@` transformations, which this build still does not have. The rule
	// being pinned is not about any particular operator: one that is not
	// implemented has to say so.
	// When
	status, stdout, stderr := runSetScript(t, "x=abc\necho [${x@Q}]\n")

	// Then
	if status != 2 || stdout != "" {
		t.Fatalf("status = %d, stdout = %q, want 2 and no output", status, stdout)
	}
	if !strings.Contains(stderr, "bad substitution") {
		t.Fatalf("stderr = %q, want a bad-substitution diagnostic", stderr)
	}
}
