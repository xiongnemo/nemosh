package runtime_test

import "testing"

// An element assignment's value takes its tilde-prefixes as an assignment's does, at its start
// and after each unquoted `:`, as bash has it; busybox-w32 has no arrays. The element was
// written with the tilde still in it, whether its subscript was plain text or not.
func TestRuntime_elementAssignmentValueExpandsTilde(t *testing.T) {
	for index, test := range []struct{ script, want string }{
		{`HOME=/h; a[0]=~/x; echo "${a[0]}"`, "/h/x\n"},
		{`HOME=/h; a[0]=x:~/y; echo "${a[0]}"`, "x:/h/y\n"},
		{`HOME=/h; declare -A A; A[k]=~; echo "${A[k]}"`, "/h\n"},
		{`HOME=/h; a[1]=foo:~; a[1]+=:~; echo "${a[1]}"`, "foo:/h:/h\n"},
		{`HOME=/h; a[0]='~'; a[1]=\~; a[2]=x~; a[3]=x=~; echo "${a[@]}"`, "~ ~ x~ x=~\n"},
		// A subscript quoted or expanded leaves the value's `=` a part or two on, and the
		// subscript itself takes no tilde after a `:`.
		{`HOME=/h; declare -A A; A['x']=foo:~; echo "${A['x']}"`, "foo:/h\n"},
		{`HOME=/h; a["0"]=~/d; k=1; a[$k]=~:x:~; echo "${a[0]} ${a[1]}"`, "/h/d /h:x:/h\n"},
		{`HOME=/h; declare -A A; k=k; A[$k:~]=v; echo "${!A[@]}"`, "k:~\n"},
	} {
		if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
			t.Errorf("%d: %s: got %q/%d, want %q/0, as bash answers", index, test.script, stdout, status, test.want)
		}
	}
}
