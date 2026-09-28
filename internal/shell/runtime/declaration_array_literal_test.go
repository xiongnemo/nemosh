package runtime_test

import "testing"

// A declaration utility's operand written as an array literal makes an array, readonly and
// export too, and one that only expands to text like it -- a quoted `x='(a b)'` -- makes a
// string, unless -a or -A asks for an array, as bash 5.3 has it; busybox-w32 has no arrays.
// The two were the same text by the time the builtin saw them, so declare and local took both
// for arrays and readonly and export took both for strings.
func TestRuntime_declarationArrayLiteralAsWritten(t *testing.T) {
	for index, test := range []struct{ script, want string }{
		{`declare "x=(a b)"; declare -p x`, "declare -- x=\"(a b)\"\n"},
		{`declare x='(a b)'; declare -p x`, "declare -- x=\"(a b)\"\n"},
		{`declare -a x="(a b)"; declare -p x`, "declare -a x=([0]=\"a\" [1]=\"b\")\n"},
		{`declare x=(a b); declare -p x`, "declare -a x=([0]=\"a\" [1]=\"b\")\n"},
		{`readonly r=(r e); declare -p r`, "declare -ar r=([0]=\"r\" [1]=\"e\")\n"},
		{`export e=(e x); declare -p e`, "declare -ax e=([0]=\"e\" [1]=\"x\")\n"},
		{`x=(1); export x+=(2); declare -p x`, "declare -ax x=([0]=\"1\" [1]=\"2\")\n"},
		{`f() { local l=(l o); declare -p l; }; f`, "declare -a l=([0]=\"l\" [1]=\"o\")\n"},
		{`f() { local l='(l o)'; declare -p l; }; f`, "declare -- l=\"(l o)\"\n"},
		{`readonly s='(r e)'; declare -p s`, "declare -r s=\"(r e)\"\n"},
	} {
		if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
			t.Errorf("%d: %s: got %q/%d, want %q/0, as bash answers", index, test.script, stdout, status, test.want)
		}
	}
}
