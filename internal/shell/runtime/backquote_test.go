package runtime_test

import "testing"

func TestRuntime_substitutesTheOutput_whenACommandIsInBackquotes(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "echo `echo inner`\n")

	// Then
	if status != 0 || stdout != "inner\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "inner\n")
	}
}

func TestRuntime_substitutesIntoAnAssignment_whenTheValueIsInBackquotes(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "x=`echo value`\necho [$x]\n")

	// Then
	if status != 0 || stdout != "[value]\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "[value]\n")
	}
}

func TestRuntime_substitutesInsideDoubleQuotes_whenTheCommandIsInBackquotes(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "echo \"[`echo quoted`]\"\n")

	// Then
	if status != 0 || stdout != "[quoted]\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "[quoted]\n")
	}
}

func TestRuntime_keepsBackquotesAsData_whenTheyAreSingleQuoted(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "echo 'a `echo no` b'\n")

	// Then
	if status != 0 || stdout != "a `echo no` b\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and the backquotes intact", status, stdout)
	}
}

func TestRuntime_keepsABackquoteAsData_whenItIsEscaped(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "echo \\`literal\\`\n")

	// Then
	if status != 0 || stdout != "`literal`\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "`literal`\n")
	}
}

func TestRuntime_runsASequentialList_whenItIsInBackquotes(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "echo \"`echo a; echo b`\"\n")

	// Then
	if status != 0 || stdout != "a\nb\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "a\nb\n")
	}
}

func TestRuntime_expandsTheInnerParameter_whenABackslashProtectsItFromTheOuterPass(t *testing.T) {
	// Inside backquotes a backslash is special only before $, another
	// backquote, and itself (POSIX 2.6.3), so `\$x` hands a live $x to the
	// inner command rather than expanding it in the outer one.
	// When
	status, stdout, _ := runSetScript(t, "x=outer\necho `x=inner; echo \\$x`\n")

	// Then
	if status != 0 || stdout != "inner\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "inner\n")
	}
}

func TestRuntime_nestsBackquotes_whenTheInnerPairIsEscaped(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "echo `echo \\`echo deep\\``\n")

	// Then
	if status != 0 || stdout != "deep\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "deep\n")
	}
}

func TestRuntime_reportsIncompleteInput_whenABackquoteIsNeverClosed(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "echo `echo unterminated\n")

	// Then
	if status != 2 || stdout != "" {
		t.Fatalf("status = %d, stdout = %q, want 2 and no output", status, stdout)
	}
}

func TestRuntime_leavesTheStatusOfTheOuterCommand_whenABackquotedCommandFails(t *testing.T) {
	// The status of a substitution belongs to the command it feeds, not to the
	// command inside it.
	// When
	status, stdout, _ := runSetScript(t, "echo [`false`]\necho [$?]\n")

	// Then
	if status != 0 || stdout != "[]\n[0]\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "[]\n[0]\n")
	}
}
