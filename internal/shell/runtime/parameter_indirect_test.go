package runtime_test

import "testing"

// `${!ref}` names any parameter, and the operator after it applies to the parameter named:
// `${!#}` is the last positional parameter, ref may be `@`, a digit, an array element or a
// whole array, and `${!ref-default}`, `${!ref#x}`, `${!ref:1:2}` and `${!ref:=x}` work on
// what ref names. Only a plain variable name was followed, and the operator was dropped. The
// answers are bash 5.3's; busybox-w32 has no indirection.
func TestRuntime_indirectionNamesAnyParameter(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`set -- a b c; echo ${!#}`, "c\n"},
		{`set -- x; x=5; echo ${!1}`, "5\n"},
		{`ref=@; set -- one two; echo "${!ref}"`, "one two\n"},
		{`ref=1; set -- one two; echo "${!ref}"`, "one\n"},
		{`a=(x y z); ref='a[1]'; echo "${!ref}"`, "y\n"},
		{`a=(x y z); ref='a[@]'; printf '[%s]' "${!ref}"`, "[x][y][z]"},
		{`ref=undef; echo "x=${!ref-default}"; foo=; ref=foo; echo "x=${!ref-default}"`, "x=default\nx=\n"},
		{`ref=v; v=value; echo "${!ref#v}" "${!ref/a/X}" "${!ref:1:2}"`, "alue vXlue al\n"},
		{`ref=w; echo "${!ref:=assigned}" "$w"`, "assigned assigned\n"},
		{`set -- a b; echo "${!#-d}" "${!1-z}"`, "b z\n"},
		{`ref=undef; echo "[${!ref}]"`, "[]\n"},
		// Unchanged: a plain name, an array's keys and the names with a prefix.
		{`a=(x y); b=a; echo "${!b}"; echo "${!a[@]}"; ab1=1 ab2=2; echo ${!ab@}`, "x\n0 1\nab1 ab2\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
	// A ref that is not set, or whose value names no parameter, stops the script.
	for _, script := range []string{`unset ref; echo "${!ref}"; echo reached`, `ref='a b'; echo "${!ref}"; echo reached`, `ref=''; echo "${!ref}"; echo reached`} {
		if stdout, status := runScriptCapturing(script); stdout != "" || status == 0 {
			t.Errorf("%s: got %q/%d, want the script stopped", script, stdout, status)
		}
	}
}
