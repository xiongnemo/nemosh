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

func TestScanShellTokens_classifiesActiveAmpersandWithoutChangingProtectedSpellings(t *testing.T) {
	// Given / When
	tokens, err := scanShellTokens(`a&&b & c <&0 >&1 "&" \&`)

	// Then
	if err != nil {
		t.Fatal(err)
	}
	want := []tokenKind{tokenWord, tokenAndIf, tokenWord, tokenBackground, tokenWord, tokenRedirect, tokenWord, tokenRedirect, tokenWord, tokenWord, tokenWord}
	if len(tokens) != len(want) {
		t.Fatalf("tokens = %#v, want %d", lexicalTokens(tokens), len(want))
	}
	for index, kind := range want {
		if tokens[index].kind != kind {
			t.Fatalf("token %d = %#v, want kind %v", index, tokens[index], kind)
		}
	}
}

func TestParseScript_marksCompleteAndOrListsAsBackgroundInSourceOrder(t *testing.T) {
	// Given / When
	script, err := ParseScript("a | b && c & d\n")

	// Then
	if err != nil {
		t.Fatal(err)
	}
	parsed := script.program[0].(listNode).value
	if len(parsed.items) != 2 {
		t.Fatalf("list items = %#v, want 2", parsed.items)
	}
	if !parsed.items[0].background || parsed.items[1].background {
		t.Fatalf("background markers = %#v", parsed.items)
	}
	if len(parsed.items[0].value.pipelines) != 2 || len(parsed.items[0].value.operators) != 1 {
		t.Fatalf("first and-or = %#v", parsed.items[0].value)
	}
}

func TestParseScript_acceptsBackgroundOnNestedTypedCommands(t *testing.T) {
	tests := []string{
		`{ echo group; } & echo next`,
		`(echo subshell) & echo next`,
		`f() { echo function; }` + "\n" + `f & echo next`,
		`echo $(echo substitution &) & echo next`,
		"cat <<EOF & echo next\nbody\nEOF\n",
	}
	for _, source := range tests {
		t.Run(source, func(t *testing.T) {
			// When
			_, err := ParseScript(source)

			// Then
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestParseScript_treatsTrailingBackgroundAsCompleteInteractiveInput(t *testing.T) {
	// When
	_, err := ParseScript("echo ready &")

	// Then
	if err != nil {
		t.Fatalf("ParseScript() error = %v, want complete input", err)
	}
}

func TestParseScript_rejectsMalformedBackgroundForms(t *testing.T) {
	tests := []string{"&", "& echo", "echo && &", "echo | &", "echo & & next", "echo &; next"}
	for _, source := range tests {
		t.Run(source, func(t *testing.T) {
			// When
			_, err := ParseScript(source)

			// Then
			if err == nil || errors.Is(err, ErrIncompleteScript) {
				t.Fatalf("ParseScript() error = %v, want malformed syntax", err)
			}
		})
	}
}

func TestRuntime_backgroundListsReturnSuccessWithoutReplacingParentStatus(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "echo first & false & echo last\nwait\n")

	// Then
	if status != 0 || !strings.Contains(stdout.String(), "first\n") || !strings.Contains(stdout.String(), "last\n") {
		t.Fatalf("status = %d, stdout = %q", status, stdout.String())
	}
}

func TestRuntime_backgroundCommandReturnsLaunchStatus(t *testing.T) {
	// Given
	rt := New(applets.DefaultRegistry, Streams{})

	// When
	status := rt.RunScript(context.Background(), "false &\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, want 0", status)
	}
}

func TestRuntime_malformedBackgroundHasNoPrefixEffects(t *testing.T) {
	// Given
	dir := t.TempDir()
	var stdout bytes.Buffer
	rt := NewWithState(applets.DefaultRegistry, Streams{Stdout: &stdout}, State{Cwd: WorkingDirectory(dir)})

	// When
	status := rt.RunScript(context.Background(), "echo prefix\nf() { echo called; }\necho created > created.txt\necho ok &; echo bad\n")

	// Then
	if status != 2 || stdout.Len() != 0 || len(rt.functions) != 0 {
		t.Fatalf("status = %d, stdout = %q, functions = %#v", status, stdout.String(), rt.functions)
	}
	if _, err := os.Stat(filepath.Join(dir, "created.txt")); !os.IsNotExist(err) {
		t.Fatalf("created.txt exists or stat failed: %v", err)
	}
}
