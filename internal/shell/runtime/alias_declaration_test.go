package runtime_test

import "testing"

// An alias for a declaration utility declares: its operands are assignments, expanded whole
// and unsplit. `e ex=$words` with `alias e=export` split the value and exported its first
// word. The answer is busybox-w32's; bash expands no alias in a script.
func TestRuntime_aliasedDeclarationUtilityDeclares(t *testing.T) {
	script := "words='a b c'\nalias e=export\nalias r=readonly\ne ex=$words\nr ro=$words\nprintf '[%s]' \"$ex\" \"$ro\""
	if stdout, status := runScriptCapturing(script); stdout != "[a b c][a b c]" || status != 0 {
		t.Errorf("got %q/%d, want %q/0, as busybox answers", stdout, status, "[a b c][a b c]")
	}
}
