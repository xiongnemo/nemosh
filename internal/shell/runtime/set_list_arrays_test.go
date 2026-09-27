package runtime_test

import "testing"

// A bare `set` lists an array as bash lists it, as the compound value that recreates it. It
// listed an indexed array as its element 0 and an associative one not at all. A scalar is
// single-quoted, as busybox lists it. The array answers are bash 5.3's; busybox has none.
func TestRuntime_setListsAnArrayAsItsCompoundValue(t *testing.T) {
	script := "a=(1 2 \"3 4\"); declare -A m=([k]=v [\"x y\"]=z); e=(); s=plain; declare -a u\n" +
		"set | grep -E '^(a|e|m|s|u)='\n"
	want := "a=([0]=\"1\" [1]=\"2\" [2]=\"3 4\")\ne=()\nm=([k]=\"v\" [\"x y\"]=\"z\" )\ns='plain'\n"
	if stdout, status := runScriptCapturing(script); stdout != want || status != 0 {
		t.Errorf("got %q/%d, want %q/0, as bash lists them", stdout, status, want)
	}
}
