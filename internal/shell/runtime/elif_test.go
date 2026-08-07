package runtime_test

import "testing"

func TestRuntime_takesTheFirstBranch_whenAnIfWithElifSucceedsFirst(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "if true; then echo a; elif true; then echo b; else echo c; fi\n")

	// Then
	if status != 0 || stdout != "a\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "a\n")
	}
}

func TestRuntime_takesTheElifBranch_whenOnlyItsConditionSucceeds(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "if false; then echo a; elif true; then echo b; else echo c; fi\n")

	// Then
	if status != 0 || stdout != "b\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "b\n")
	}
}

func TestRuntime_takesTheElseBranch_whenNoElifConditionSucceeds(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "if false; then echo a; elif false; then echo b; else echo c; fi\n")

	// Then
	if status != 0 || stdout != "c\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "c\n")
	}
}

func TestRuntime_choosesAmongSeveral_whenAnIfHasMoreThanOneElif(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t,
		"if false; then echo a; elif false; then echo b; elif true; then echo c; else echo d; fi\n")

	// Then
	if status != 0 || stdout != "c\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "c\n")
	}
}

func TestRuntime_fallsThroughToNothing_whenAnElifChainHasNoElse(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "if false; then echo a; elif false; then echo b; fi\necho after\n")

	// Then
	if status != 0 || stdout != "after\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "after\n")
	}
}

func TestRuntime_acceptsElifAcrossLines_whenTheIfIsWrittenOut(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "if false\nthen\necho a\nelif true\nthen\necho b\nelse\necho c\nfi\n")

	// Then
	if status != 0 || stdout != "b\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "b\n")
	}
}

func TestRuntime_acceptsElif_whenItIsNestedInALoop(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t,
		"for i in 1 2; do if test $i = 1; then echo one; elif test $i = 2; then echo two; fi; done\n")

	// Then
	if status != 0 || stdout != "one\ntwo\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "one\ntwo\n")
	}
}

func TestRuntime_acceptsNestedElifChains_whenOneIfSitsInsideAnother(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t,
		"if true; then if false; then echo x; elif true; then echo inner; fi; elif true; then echo y; fi\n")

	// Then
	if status != 0 || stdout != "inner\n" {
		t.Fatalf("status = %d, stdout = %q, want 0 and %q", status, stdout, "inner\n")
	}
}

func TestRuntime_reportsSyntaxError_whenABraceExpansionIsNeverClosed(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "echo ${x\n")

	// Then
	if status != 2 || stdout != "" {
		t.Fatalf("status = %d, stdout = %q, want 2 and no output", status, stdout)
	}
	if stderr == "" {
		t.Fatalf("stderr = %q, want a diagnostic", stderr)
	}
}

func TestRuntime_reportsSyntaxError_whenABraceExpansionWithAnOperatorIsNeverClosed(t *testing.T) {
	// When
	status, stdout, _ := runSetScript(t, "echo ${x:-fallback\n")

	// Then
	if status != 2 || stdout != "" {
		t.Fatalf("status = %d, stdout = %q, want 2 and no output", status, stdout)
	}
}
