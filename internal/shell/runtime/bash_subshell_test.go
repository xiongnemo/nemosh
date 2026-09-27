package runtime_test

import "testing"

// BASH_SUBSHELL is bash's count of the subshells around a command: a `( )`, a command or
// process substitution, a background job. A pipeline's stages are not counted, in bash. It
// was unset. busybox has not got it.
func TestBashSubshell(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "echo \"a $BASH_SUBSHELL\"\n"+
		"(echo \"b $BASH_SUBSHELL\")\n"+
		"echo \"c $(echo $BASH_SUBSHELL)\"\n"+
		"( (echo \"d $BASH_SUBSHELL\") )\n"+
		"echo x | echo \"e $BASH_SUBSHELL\"\n"+
		"{ echo \"f $BASH_SUBSHELL\"; }\n"+
		"g() { echo \"g $BASH_SUBSHELL\"; }\ng\n"+
		"echo \"h $BASH_SUBSHELL\" &\nwait\n"+
		"cat <(echo \"j $BASH_SUBSHELL\")\n"+
		"echo \"k $( (echo $BASH_SUBSHELL) )\"\n")

	// Then
	want := "a 0\nb 1\nc 1\nd 2\ne 0\nf 0\ng 0\nh 1\nj 1\nk 2\n"
	if stdout != want || status != 0 {
		t.Fatalf("got %q/%d, want %q/0; stderr = %q", stdout, status, want, stderr)
	}
}
