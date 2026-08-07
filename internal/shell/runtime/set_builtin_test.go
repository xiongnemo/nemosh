package runtime_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func runSetScript(t *testing.T, source string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})
	return rt.RunScript(context.Background(), source), stdout.String(), stderr.String()
}

func TestRuntime_expandsABracedName_whenItCarriesNoOperator(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "x=value\necho [${x}]\n")

	// Then
	if status != 0 || stdout != "[value]\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "[value]\n")
	}
}

func TestRuntime_expandsABracedNameToEmpty_whenItIsUnset(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "echo [${nosuch}]\n")

	// Then
	if status != 0 || stdout != "[]\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "[]\n")
	}
}

func TestRuntime_joinsABracedNameToText_whenTheBracesDelimitIt(t *testing.T) {
	// The braces exist to end the name early, which is the only way to write
	// this at all.
	// When
	status, stdout, _ := runSetScript(t, "x=a\necho ${x}bc\n")

	// Then
	if status != 0 || stdout != "abc\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "abc\n")
	}
}

func TestRuntime_replacesThePositionalParameters_whenSetIsGivenOperands(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "set a b c\necho $# $1 $2 $3\n")

	// Then
	if status != 0 || stdout != "3 a b c\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "3 a b c\n")
	}
}

func TestRuntime_keepsThePositionalParameters_whenSetOnlyChangesAnOption(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "set a b c\nset -f\necho $#\n")

	// Then
	if status != 0 || stdout != "3\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "3\n")
	}
}

func TestRuntime_clearsThePositionalParameters_whenSetIsGivenOnlyADoubleDash(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "set a b c\nset --\necho [$#]\n")

	// Then
	if status != 0 || stdout != "[0]\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "[0]\n")
	}
}

func TestRuntime_treatsALeadingDashOperandAsData_whenItFollowsADoubleDash(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "set -- -f -x\necho $# $1 $2\n")

	// Then
	if status != 0 || stdout != "2 -f -x\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "2 -f -x\n")
	}
}

func TestRuntime_reportsTheEnabledOptions_whenDollarDashIsExpanded(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "set -ef\necho [$-]\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, want 0", status)
	}
	if !strings.Contains(stdout, "e") || !strings.Contains(stdout, "f") {
		t.Fatalf("stdout = %q, want it to carry both e and f", stdout)
	}
}

func TestRuntime_clearsAnOption_whenTheSignIsPlus(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "set -f\nset +f\necho [$-]\n")

	// Then
	if status != 0 || strings.Contains(stdout, "f") {
		t.Fatalf("status = %d, stdout = %q, want 0 and no f", status, stdout)
	}
}

func TestRuntime_clearsPipefail_whenItIsTurnedOffByName(t *testing.T) {
	// The old implementation could only ever turn pipefail on, so this was a
	// one-way latch.
	// When
	status, stdout, _ := runSetScript(t, "set -o pipefail\nset +o pipefail\nfalse | true\necho [$?]\n")

	// Then
	if status != 0 || stdout != "[0]\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "[0]\n")
	}
}

func TestRuntime_refusesAnUnknownLetter_whenSetIsGivenOne(t *testing.T) {
	// When
	status, _, stderr := runSetScript(t, "set -Z\n")

	// Then
	if status != 2 {
		t.Fatalf("status = %d, want 2", status)
	}
	if !strings.Contains(stderr, "illegal option") || !strings.Contains(stderr, "Z") {
		t.Fatalf("stderr = %q, want an illegal-option diagnostic naming Z", stderr)
	}
}

func TestRuntime_refusesAnUnknownLongName_whenSetIsGivenOne(t *testing.T) {
	// When
	status, _, stderr := runSetScript(t, "set -o bogus\n")

	// Then
	if status != 2 {
		t.Fatalf("status = %d, want 2", status)
	}
	if !strings.Contains(stderr, "illegal option") || !strings.Contains(stderr, "bogus") {
		t.Fatalf("stderr = %q, want an illegal-option diagnostic naming bogus", stderr)
	}
}

func TestRuntime_listsTheOptionStates_whenSetIsGivenABareDashO(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "set -e\nset -o\n")

	// Then
	if status != 0 {
		t.Fatalf("status = %d, want 0", status)
	}
	if !strings.Contains(stdout, "errexit") || !strings.Contains(stdout, "on") {
		t.Fatalf("stdout = %q, want errexit reported on", stdout)
	}
	if !strings.Contains(stdout, "nounset") || !strings.Contains(stdout, "off") {
		t.Fatalf("stdout = %q, want nounset reported off", stdout)
	}
}

func TestRuntime_listsReusableCommands_whenSetIsGivenABarePlusO(t *testing.T) {
	// POSIX asks for a form that can be read back in as input.
	// When
	status, stdout, _ := runSetScript(t, "set -e\nset +o\n")

	// Then
	if status != 0 || !strings.Contains(stdout, "set -o errexit") {
		t.Fatalf("status = %d, stdout = %q, want a reusable `set -o errexit`", status, stdout)
	}
}

func TestRuntime_listsTheShellVariables_whenSetHasNoArguments(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "chosen=marker\nset\n")

	// Then
	if status != 0 || !strings.Contains(stdout, "chosen='marker'") {
		t.Fatalf("status = %d, stdout = %q, want a quoted chosen=marker", status, stdout)
	}
}
