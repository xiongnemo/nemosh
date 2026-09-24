package runtime_test

import "testing"

// A function whose body held a case and then an `||` was cut at the `||`: the pattern's `)`
// was counted as closing the body's brace, and the script failed to parse with `missing }`.
func TestFunction_withACaseKeepsItsWholeBody(t *testing.T) {
	script := "g() {\n  case $1 in\n    *) echo other ;;\n  esac\n  (echo sub) || true\n}\ng\n"
	if stdout, status := runScriptCapturing(script); status != 0 || stdout != "other\nsub\n" {
		t.Fatalf("status %d stdout %q", status, stdout)
	}
}
