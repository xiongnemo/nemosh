package runtime_test

import "testing"

// `shopt -s localvar_inherit` starts a new local with the value, the array and the
// attributes the name had where the function was called, as bash's does; without it a local
// starts with nothing, in both references. The caller's are untouched either way. busybox has
// no shopt.
func TestShopt_localvarInherit(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "shopt -s localvar_inherit\n"+
		"x=outer; declare -i n=5; a=(1 2)\n"+
		"f() { local x n a; echo \"[$x][$n][${a[*]}]\"; n=2+3; a[0]=9; x=changed; echo \"[$n][${a[*]}][$x]\"; }\n"+
		"f\necho \"[$x][$n][${a[*]}]\"\nshopt -u localvar_inherit\nf\n")

	// Then
	want := "[outer][5][1 2]\n[5][9 2][changed]\n[outer][5][1 2]\n[][][]\n[2+3][9][changed]\n"
	if stdout != want || status != 0 {
		t.Fatalf("got %q/%d, want %q/0; stderr = %q", stdout, status, want, stderr)
	}
}
