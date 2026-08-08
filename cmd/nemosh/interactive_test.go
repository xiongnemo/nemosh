package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

type interactiveOutcome struct {
	stdout string
	stderr string
	err    error
}

func runInteractiveTest(input io.Reader) interactiveOutcome {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := command{stdin: input, stdout: &stdout, stderr: &stderr}
	err := cmd.run(context.Background(), []string{"nemosh", "-i"})
	return interactiveOutcome{stdout: stdout.String(), stderr: stderr.String(), err: err}
}

func interactiveStatus(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	var status exitStatus
	if !errors.As(err, &status) {
		t.Fatalf("interactive error = %v, want exit status", err)
	}
	return int(status)
}

func TestRunInteractive_writesPrimaryPromptOnlyToStderr(t *testing.T) {
	// Given / When
	got := runInteractiveTest(strings.NewReader("exit 0\n"))

	// Then
	plain := withoutANSI(got.stderr)
	if got.stdout != "" || !strings.HasPrefix(plain, "# ") || !strings.HasSuffix(plain, "\n"+promptSymbol()+" ") {
		t.Fatalf("streams = (%q, %q), want informative default prompt on stderr", got.stdout, got.stderr)
	}
}

func TestRunInteractive_usesAssignedPS1AndPS2(t *testing.T) {
	// Given / When
	got := runInteractiveTest(strings.NewReader("PS1='primary> '\nPS2='continue> '\necho one\\\ntwo\nexit 0\n"))

	// Then
	if got.stdout != "onetwo\n" {
		t.Fatalf("stdout = %q, want %q", got.stdout, "onetwo\n")
	}
	if !strings.Contains(got.stderr, "primary> continue> primary> ") {
		t.Fatalf("stderr = %q, want assigned primary and continuation prompts", got.stderr)
	}
}

func TestRunInteractive_preservesExplicitEmptyPS1(t *testing.T) {
	// Given / When
	got := runInteractiveTest(strings.NewReader("PS1=''\nexit 0\n"))

	// Then
	if got.stdout != "" || strings.Count(withoutANSI(got.stderr), "\n"+promptSymbol()+" ") != 1 {
		t.Fatalf("streams = (%q, %q), want no fallback prompt after explicit empty PS1", got.stdout, got.stderr)
	}
}

func TestRunInteractive_writesContinuationPromptOnlyToStderr(t *testing.T) {
	// Given / When
	got := runInteractiveTest(strings.NewReader("echo one\\\ntwo\nexit 0\n"))

	// Then
	if got.stdout != "onetwo\n" || strings.Count(withoutANSI(got.stderr), "> ") != 1 || strings.Count(withoutANSI(got.stderr), "\n"+promptSymbol()+" ") != 2 {
		t.Fatalf("streams = (%q, %q), want two primary prompts and one continuation prompt", got.stdout, got.stderr)
	}
}

func TestRunInteractive_returnsSavedStatusAtEOF(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		status int
	}{
		{name: "failure", input: "false\n", status: 1},
		{name: "success", input: "true\n", status: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given / When
			got := runInteractiveTest(strings.NewReader(tt.input))

			// Then
			if status := interactiveStatus(t, got.err); status != tt.status {
				t.Fatalf("status = %d, want %d", status, tt.status)
			}
		})
	}
}

func TestRunInteractive_bareExitUsesSavedStatus(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		status int
	}{
		{name: "bare exit", input: "false\nexit\n", status: 1},
		{name: "explicit exit", input: "false\nexit 7\n", status: 7},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given / When
			got := runInteractiveTest(strings.NewReader(test.input))

			// Then
			if status := interactiveStatus(t, got.err); status != test.status {
				t.Fatalf("status = %d, want %d", status, test.status)
			}
		})
	}
}

func TestRunInteractive_compoundBodyBareExitUsesSavedStatus(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "case", input: "false\ncase selected in\nselected)\nexit\nesac\n"},
		{name: "for", input: "false\nfor item in one\ndo\nexit\ndone\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given / When
			got := runInteractiveTest(strings.NewReader(test.input))

			// Then
			if status := interactiveStatus(t, got.err); status != 1 {
				t.Fatalf("status = %d, want 1", status)
			}
		})
	}
}

func TestRunInteractive_commentWithUnmatchedQuoteDoesNotContinue(t *testing.T) {
	// Given / When
	got := runInteractiveTest(strings.NewReader("# unmatched '\necho ran\n"))

	// Then
	if got.stdout != "ran\n" || strings.Contains(got.stderr, "> ") {
		t.Fatalf("streams = (%q, %q), want command output and primary prompts", got.stdout, got.stderr)
	}
}

func TestRunInteractive_executesMultilineLexicalFormsOnce(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		stdout string
	}{
		{name: "backslash", input: "echo one\\\ntwo\n", stdout: "onetwo\n"},
		{name: "single quote", input: "printf '%s' 'one\ntwo'\n", stdout: "one\ntwo"},
		{name: "double quote", input: "printf '%s' \"one\ntwo\"\n", stdout: "one\ntwo"},
		{name: "command substitution", input: "echo $(\necho nested\n)\n", stdout: "nested\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given / When
			got := runInteractiveTest(strings.NewReader(tt.input))

			// Then
			if got.stdout != tt.stdout {
				t.Fatalf("stdout = %q, want %q", got.stdout, tt.stdout)
			}
			if strings.Count(withoutANSI(got.stderr), "> ") == 0 {
				t.Fatalf("stderr = %q, want continuation prompt", got.stderr)
			}
		})
	}
}

func TestRunInteractive_continuesTrailingPipelineAcrossPhysicalLines(t *testing.T) {
	// Given / When
	got := runInteractiveTest(strings.NewReader("echo piped |\ncat\n"))

	// Then
	if got.stdout != "piped\n" || interactiveStatus(t, got.err) != 0 {
		t.Fatalf("outcome = %+v, want piped output and status 0", got)
	}
	if strings.Count(withoutANSI(got.stderr), "> ") != 1 {
		t.Fatalf("stderr = %q, want one continuation prompt", got.stderr)
	}
}

func TestRunInteractive_continuesTrailingRedirectAcrossPhysicalLines(t *testing.T) {
	// Given / When
	got := runInteractiveTest(strings.NewReader("echo redirected >\n/dev/null\n"))

	// Then
	if got.stdout != "" || interactiveStatus(t, got.err) != 0 {
		t.Fatalf("outcome = %+v, want redirected output and status 0", got)
	}
	if strings.Count(withoutANSI(got.stderr), "> ") != 1 {
		t.Fatalf("stderr = %q, want one continuation prompt", got.stderr)
	}
}

func TestRunInteractive_executesNestedIfWithoutLeakingCloser(t *testing.T) {
	// Given
	input := "if true\nthen\nif true\nthen\necho nested\nfi\nfi\n"

	// When
	got := runInteractiveTest(strings.NewReader(input))

	// Then
	if got.stdout != "nested\n" || strings.Contains(got.stderr, "unexpected fi") {
		t.Fatalf("streams = (%q, %q), want one nested execution", got.stdout, got.stderr)
	}
}

func TestRunInteractive_reportsMalformedSyntaxOnceAndResets(t *testing.T) {
	// Given / When
	got := runInteractiveTest(strings.NewReader("fi\necho recovered\n"))

	// Then
	if got.stdout != "recovered\n" {
		t.Fatalf("stdout = %q, want recovered command output", got.stdout)
	}
	if count := strings.Count(got.stderr, "nemosh: syntax error:"); count != 1 {
		t.Fatalf("diagnostic count = %d in %q, want 1", count, got.stderr)
	}
}

func TestRunInteractive_rejectsMalformedInteriorPipelineAndRecovers(t *testing.T) {
	// Given / When
	got := runInteractiveTest(strings.NewReader("a | | b\necho recovered\n"))

	// Then
	if got.stdout != "recovered\n" {
		t.Fatalf("stdout = %q, want recovered command output", got.stdout)
	}
	if strings.Contains(got.stderr, "> ") {
		t.Fatalf("stderr = %q, malformed pipeline must not continue", got.stderr)
	}
	if count := strings.Count(got.stderr, "nemosh:"); count != 1 {
		t.Fatalf("diagnostic count = %d in %q, want 1", count, got.stderr)
	}
}

func TestRunInteractive_malformedFinalLineReturnsTwo(t *testing.T) {
	// Given / When
	got := runInteractiveTest(strings.NewReader("fi"))

	// Then
	if status := interactiveStatus(t, got.err); status != 2 {
		t.Fatalf("status = %d, want 2", status)
	}
	if count := strings.Count(got.stderr, "nemosh: syntax error:"); count != 1 {
		t.Fatalf("diagnostic count = %d in %q, want 1", count, got.stderr)
	}
}

func TestRunInteractive_incompleteEOFReportsOnceAndReturnsTwo(t *testing.T) {
	// Given / When
	got := runInteractiveTest(strings.NewReader("if true\nthen\n"))

	// Then
	if status := interactiveStatus(t, got.err); status != 2 {
		t.Fatalf("status = %d, want 2", status)
	}
	if count := strings.Count(got.stderr, "nemosh: unexpected end of file"); count != 1 {
		t.Fatalf("diagnostic count = %d in %q, want 1", count, got.stderr)
	}
}

func TestRunInteractive_runsExitTrapOnceOnEOFOrExit(t *testing.T) {
	for _, input := range []string{"trap 'echo trapped' EXIT\n", "trap 'echo trapped' EXIT\nexit 0\n"} {
		// Given / When
		got := runInteractiveTest(strings.NewReader(input))

		// Then
		if count := strings.Count(got.stdout, "trapped\n"); count != 1 {
			t.Fatalf("trap count = %d in %q, want 1", count, got.stdout)
		}
	}
}

func TestRunInteractive_execSkipsExitTrap(t *testing.T) {
	// Given / When
	got := runInteractiveTest(strings.NewReader("trap 'echo trapped' EXIT\nexec true\n"))

	// Then
	if got.stdout != "" {
		t.Fatalf("stdout = %q, want no EXIT trap output", got.stdout)
	}
}

func TestRunInteractive_readConsumesNextPhysicalLine(t *testing.T) {
	// Given / When
	got := runInteractiveTest(strings.NewReader("read name\nnemo\necho $name\nexit 0\n"))

	// Then
	if got.stdout != "nemo\n" {
		t.Fatalf("stdout = %q, want %q", got.stdout, "nemo\n")
	}
}

func TestRunInteractive_executesFinalLineWithoutNewline(t *testing.T) {
	// Given / When
	got := runInteractiveTest(strings.NewReader("echo final"))

	// Then
	if got.stdout != "final\n" || interactiveStatus(t, got.err) != 0 {
		t.Fatalf("outcome = %+v, want final output and status 0", got)
	}
}
