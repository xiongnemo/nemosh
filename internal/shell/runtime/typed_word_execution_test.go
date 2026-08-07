package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestTypedCommandSubstitution_executesParsedScript_whenSourceTextDiffers(t *testing.T) {
	// Given
	script, err := ParseScript("echo $(echo source)\n")
	if err != nil {
		t.Fatal(err)
	}
	command := script.program[0].(listNode).value.items[0].value.pipelines[0].commands[0].(simpleCommand)
	part := &command.words[1].parts[0]
	nested := part.script.program[0].(listNode)
	nestedCommand := nested.value.items[0].value.pipelines[0].commands[0].(simpleCommand)
	nestedCommand.words[1] = word{
		parts: []wordPart{{kind: wordPartLiteral, text: "typed"}},
	}
	nested.value.items[0].value.pipelines[0].commands[0] = nestedCommand
	part.script.program[0] = nested
	var stdout bytes.Buffer
	runtime := New(applets.DefaultRegistry, Streams{Stdout: &stdout})

	// When
	status, control := runtime.executePrepared(context.Background(), script)

	// Then
	if status != 0 || control != flowNone {
		t.Fatalf("executePrepared() = (%d, %d), want (0, %d)", status, control, flowNone)
	}
	if got := stdout.String(); got != "typed\n" {
		t.Fatalf("stdout = %q, want %q", got, "typed\n")
	}
}

func TestTypedWordExpansion_preservesPositionalParameterCardinality(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	runtime := New(applets.DefaultRegistry, Streams{Stdout: &stdout})

	// When
	status := runtime.RunScript(context.Background(), "set -- first second\necho $@\n")

	// Then
	if status != 0 {
		t.Fatalf("RunScript() status = %d, want 0", status)
	}
	if got := stdout.String(); got != "first second\n" {
		t.Fatalf("stdout = %q, want %q", got, "first second\n")
	}
}

func TestTypedRedirectExpansion_rejectsMultiplePositionalParameters(t *testing.T) {
	// Given
	var stderr bytes.Buffer
	directory := t.TempDir()
	first := filepath.ToSlash(filepath.Join(directory, "first"))
	second := filepath.ToSlash(filepath.Join(directory, "second"))
	runtime := New(applets.DefaultRegistry, Streams{Stderr: &stderr})

	// When
	status := runtime.RunScript(context.Background(), "set -- "+first+" "+second+"\necho value > $@\n")

	// Then
	if status != 1 {
		t.Fatalf("RunScript() status = %d, want 1", status)
	}
	if _, err := os.Stat(first); !os.IsNotExist(err) {
		t.Fatalf("first redirect target unexpectedly exists: %v", err)
	}
	if _, err := os.Stat(second); !os.IsNotExist(err) {
		t.Fatalf("second redirect target unexpectedly exists: %v", err)
	}
	if got := stderr.String(); got == "" {
		t.Fatal("stderr is empty, want ambiguous redirect diagnostic")
	}
}

func TestTypedCompoundExpansion_expandsForValuesAndCasePatterns(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	runtime := New(applets.DefaultRegistry, Streams{Stdout: &stdout})

	// When
	status := runtime.RunScript(context.Background(), "set -- alpha beta\nfor value in $@\ndo\ncase $value in\n$value)\necho $value\n;;\nesac\ndone\n")

	// Then
	if status != 0 {
		t.Fatalf("RunScript() status = %d, want 0", status)
	}
	if got := stdout.String(); got != "alpha\nbeta\n" {
		t.Fatalf("stdout = %q, want %q", got, "alpha\nbeta\n")
	}
}

func TestTypedWordExpansion_preservesConcatenatedEmptyQuotes(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	runtime := New(applets.DefaultRegistry, Streams{Stdout: &stdout})

	// When
	status := runtime.RunScript(context.Background(), "echo a''b ''\n")

	// Then
	if status != 0 {
		t.Fatalf("RunScript() status = %d, want 0", status)
	}
	if got := stdout.String(); got != "ab \n" {
		t.Fatalf("stdout = %q, want %q", got, "ab \n")
	}
}

func TestTypedParameterExpansion_preservesSupportedForms(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	runtime := New(applets.DefaultRegistry, Streams{Stdout: &stdout})

	// When
	status := runtime.RunScript(context.Background(), "set -- one 'two words'\nfalse\necho $?-$#-$1-$2\necho $*\necho ${missing:-fallback}\nfalse\necho ${missing:-$?}\necho ${missing:-$*}\necho \\$1\n")

	// Then
	if status != 0 {
		t.Fatalf("RunScript() status = %d, want 0", status)
	}
	want := "1-2-one-two words\none two words\nfallback\n1\none two words\n$1\n"
	if got := stdout.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestTypedParameterExpansion_preservesQuotedAtCardinality(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	runtime := New(applets.DefaultRegistry, Streams{Stdout: &stdout})

	// When
	status := runtime.RunScript(context.Background(), "set -- one 'two words'\nprintf '[%s]\\n' \"$@\"\n")

	// Then
	if status != 0 {
		t.Fatalf("RunScript() status = %d, want 0", status)
	}
	// Two fields, so printf reuses its format once per operand. The expectation
	// used to be Go's `%!(EXTRA string=...)`, which was an artifact of handing
	// the whole thing to fmt.Fprintf rather than implementing the utility.
	if got := stdout.String(); got != "[one]\n[two words]\n" {
		t.Fatalf("stdout = %q, want distinct quoted positional arguments", got)
	}
}

func TestTypedParameterExpansion_omitsQuotedAtWhenNoParametersExist(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	runtime := New(applets.DefaultRegistry, Streams{Stdout: &stdout})

	// When
	status := runtime.RunScript(context.Background(), "printf '<%s>\\n' \"$@\"\n")

	// Then
	if status != 0 {
		t.Fatalf("RunScript() status = %d, want 0", status)
	}
	// No parameters means no fields, so the format runs once with nothing to
	// substitute and %s is empty -- not Go's `%!s(MISSING)`.
	if got := stdout.String(); got != "<>\n" {
		t.Fatalf("stdout = %q, want no positional argument", got)
	}
}
