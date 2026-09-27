package runtime_test

import "testing"

// An array subscript is arithmetic, and an error in it is an arithmetic error, which ends the
// script as `$((1+))` does -- bash's answer, busybox having no arrays -- and in a subshell
// ends only the subshell. It was said and passed over: an assignment went on with status 0,
// a read expanded to nothing, and unset returned 1. A blank subscript is 0, as bash 5.3's
// empty expression is, where it was "array subscript: empty".
func TestRuntime_arraySubscriptErrorsAreArithmeticErrors(t *testing.T) {
	tests := []struct {
		script, want string
		status       int
	}{
		{"a=(1); a[1+]=2; echo next", "", 2},
		{"a=(1); echo ${a[1+]}; echo next", "", 2},
		{"a=(1 2); unset 'a[1+]'; echo next", "", 2},
		{"a=(5 6); a[1/0]=7; echo next", "", 2},
		{"( a[1+]=2; echo sub ); echo \"after $?\"", "after 2\n", 0},
		{"a=(5 6); a[\" \"]=7; echo \"${a[@]}\"", "7 6\n", 0},
	}
	// A subscript that comes to nothing leaves the word an assignment, as it was written: `a[$e]=1`
	// with e empty expanded to `a[]=1`, no longer an assignment's shape, and was run as a
	// command of that name. An indexed array's element 0 is assigned, and an associative
	// array refuses the empty key and ends the script, as bash has both.
	tests = append(tests, []struct {
		script, want string
		status       int
	}{
		{"e=; a[$e]=1; a[$e]+=2; echo \"st=$? ${a[0]}\"", "st=0 12\n", 0},
		{"a[\"\"]=1; echo \"st=$? ${a[0]}\"", "st=0 1\n", 0},
		{"e=; a=(1 2); a[$e]=x b=y; echo \"st=$? ${a[@]} $b\"", "st=0 x 2 y\n", 0},
		{"declare -A m; k=; m[$k]=1; echo next", "", 2},
		{"declare -A m; m[\"\"]=1; echo next", "", 2},
	}...)
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != test.status {
				t.Errorf("got %q/%d, want %q/%d", stdout, status, test.want, test.status)
			}
		})
	}
}
