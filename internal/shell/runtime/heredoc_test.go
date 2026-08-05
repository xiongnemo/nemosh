package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestParseScript_preservesTypedHeredocMetadataAndFIFOBody(t *testing.T) {
	// Given
	source := "cat 3<<'A' <<-B\nfirst # literal\nA\n\tsecond\n\tB\n"

	// When
	script, err := ParseScript(source)

	// Then
	if err != nil {
		t.Fatal(err)
	}
	redirects := firstSimpleCommand(t, script).redirects
	if len(redirects) != 2 {
		t.Fatalf("redirects = %#v, want 2", redirects)
	}
	first, second := redirects[0], redirects[1]
	if first.kind != redirectHeredoc || first.target != 3 || first.delimiter != "A" || first.expand || first.stripTabs || first.body != "first # literal\n" {
		t.Fatalf("first heredoc = %#v", first)
	}
	if second.kind != redirectHeredoc || second.target != 0 || second.delimiter != "B" || !second.expand || !second.stripTabs || second.body != "second\n" {
		t.Fatalf("second heredoc = %#v", second)
	}
}

func TestRuntime_RunScript_executesHeredocExpansionQuotingAndTabStripping(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{name: "unquoted expands without splitting", source: "VALUE='a b'\ncat <<EOF\n$VALUE $(printf sub)\nEOF\n", want: "a b sub\n"},
		{name: "single quoted delimiter is literal", source: "VALUE=changed\ncat <<'EOF'\n$VALUE $(printf sub)\nEOF\n", want: "$VALUE $(printf sub)\n"},
		{name: "double quoted delimiter is literal", source: "cat <<\"EOF\"\n$value\nEOF\n", want: "$value\n"},
		{name: "partially quoted delimiter is literal", source: "cat <<E'OF'\n$value\nEOF\n", want: "$value\n"},
		{name: "strip tabs only", source: "cat <<-EOF\n\ttab\n space\n\tEOF\n", want: "tab\n space\n"},
		{name: "empty body", source: "cat <<EOF\nEOF\n", want: ""},
		{name: "crlf", source: "cat <<EOF\r\nvalue\r\nEOF\r\n", want: "value\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			var stdout, stderr bytes.Buffer
			rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})

			// When
			status := rt.RunScript(context.Background(), tt.source)

			// Then
			if status != 0 || stdout.String() != tt.want {
				t.Fatalf("RunScript() = status %d stdout %q stderr %q, want stdout %q", status, stdout.String(), stderr.String(), tt.want)
			}
		})
	}
}

func TestRuntime_RunScript_appliesHeredocsInLexicalOrderAcrossCommands(t *testing.T) {
	inputPath := filepath.ToSlash(filepath.Join(t.TempDir(), "input.txt"))
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{name: "later file input wins", source: fmt.Sprintf("printf 'file\\n' > %s\ncat <<EOF < %s\nheredoc\nEOF\n", inputPath, inputPath), want: "file\n"},
		{name: "later heredoc wins", source: fmt.Sprintf("printf 'file\\n' > %s\ncat < %s <<EOF\nheredoc\nEOF\n", inputPath, inputPath), want: "heredoc\n"},
		{name: "pipeline input", source: "printf pipe | cat <<EOF\nheredoc\nEOF\n", want: "heredoc\n"},
		{name: "brace group", source: "{ cat; } <<EOF\ngroup\nEOF\n", want: "group\n"},
		{name: "subshell", source: "(cat) <<EOF\nsubshell\nEOF\n", want: "subshell\n"},
		{name: "fd 3 duplicated to stdin", source: "cat 3<<EOF 0<&3\nfd-three\nEOF\n", want: "fd-three\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			var stdout, stderr bytes.Buffer
			rt := New(applets.DefaultRegistry, Streams{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr})

			// When
			status := rt.RunScript(context.Background(), tt.source)

			// Then
			if status != 0 || stdout.String() != tt.want {
				t.Fatalf("RunScript() = status %d stdout %q stderr %q, want %q", status, stdout.String(), stderr.String(), tt.want)
			}
		})
	}
}

func TestRuntime_RunScript_missingHeredocTerminatorIsIncompleteAndExecutesNoPrefix(t *testing.T) {
	// Given
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "echo before\ncat <<EOF\nbody\n")

	// Then
	if status != 2 || stdout.Len() != 0 || !errors.Is(parseScriptError(t, "cat <<EOF\nbody\n"), ErrIncompleteScript) {
		t.Fatalf("status = %d stdout = %q stderr = %q", status, stdout.String(), stderr.String())
	}
	if strings.Count(stderr.String(), "nemosh:") != 1 {
		t.Fatalf("stderr = %q, want one diagnostic", stderr.String())
	}
}

func TestRuntime_RunScript_isolatesNestedCommandSubstitutionHeredocQueue(t *testing.T) {
	// Given
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
	source := "echo $(cat <<INNER\nnested\nINNER\n)\ncat <<OUTER\nouter\nOUTER\n"

	// When
	status := rt.RunScript(context.Background(), source)

	// Then
	if status != 0 || stdout.String() != "nested\nouter\n" {
		t.Fatalf("RunScript() = status %d stdout %q stderr %q", status, stdout.String(), stderr.String())
	}
}

func TestParseScript_doesNotChargeHeredocBodyAsTokens(t *testing.T) {
	// Given
	body := strings.Repeat("word ", maxParseTokens+1)
	source := "cat <<EOF\n" + body + "\nEOF\n"

	// When
	_, err := ParseScript(source)

	// Then
	if err != nil {
		t.Fatalf("ParseScript() error = %v", err)
	}
}

func TestRuntime_RunScript_ignoresHeredocOperatorInsideComment(t *testing.T) {
	// Given
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "echo ok # <<EOF\n")

	// Then
	if status != 0 || stdout.String() != "ok\n" || stderr.Len() != 0 {
		t.Fatalf("RunScript() = status %d stdout %q stderr %q", status, stdout.String(), stderr.String())
	}
}

func TestRuntime_RunScript_associatesCompoundHeredocsInLexicalOrder(t *testing.T) {
	// Given
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
	source := "while cat <<A\nfirst\nA\ndo\ncat <<B\nsecond\nB\nbreak\ndone\n"

	// When
	status := rt.RunScript(context.Background(), source)

	// Then
	if status != 0 || stdout.String() != "first\nsecond\n" {
		t.Fatalf("RunScript() = status %d stdout %q stderr %q", status, stdout.String(), stderr.String())
	}
}

func TestRuntime_RunScript_foldsBackslashNewlineInUnquotedHeredoc(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), "cat <<EOF\na\\\nb\nEOF\n")

	// Then
	if status != 0 || stdout.String() != "ab\n" {
		t.Fatalf("RunScript() = status %d stdout %q", status, stdout.String())
	}
}

func TestRuntime_RunScript_escapesControlBytesInMissingDelimiterDiagnostic(t *testing.T) {
	// Given
	var stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "cat <<'BAD\x1b'\nbody\n")

	// Then
	if status != 2 || strings.ContainsRune(stderr.String(), '\x1b') || !strings.Contains(stderr.String(), `BAD\x1b`) {
		t.Fatalf("RunScript() = status %d stderr %q", status, stderr.String())
	}
}

func parseScriptError(t *testing.T, source string) error {
	t.Helper()
	_, err := ParseScript(source)
	return err
}
