package runtime_test

import "testing"

// A function's own prefix assignments are its locals, as ash's evalfun makes them: `v=tempenv
// f` with `local v` in f declares v again and keeps its value, still in the environment. It
// was unset. A call the body makes gets none of them, so its `local v` is unset, which is
// busybox's answer there and not bash's. busybox-w32 and bash agree on every other line here.
func TestRuntime_aCallsPrefixAssignmentsAreItsLocals(t *testing.T) {
	script := `f1() { local v; echo "[${v-(unset)}]"; }
f2() { f1; }
f3() { local v=x; echo "[$v]"; }
f5() { local v; env | grep -c '^v=tempenv$'; }
v=global
v=tempenv f1
v=tempenv f2
f1
v=tempenv f3
echo "[$v]"
v=tempenv f5
`
	want := "[tempenv]\n[(unset)]\n[(unset)]\n[x]\n[global]\n1\n"
	if stdout, status := runScriptCapturing(script); stdout != want || status != 0 {
		t.Errorf("got %q/%d, want %q/0", stdout, status, want)
	}
}
