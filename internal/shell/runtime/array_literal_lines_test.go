package runtime_test

import "testing"

// An array literal can run over several lines, and a newline between its elements separates
// them as a blank does. It separated nothing: the newline stayed in the next element, so
// `a=(\n1\n'2 3'\n)` made two elements of which the second was "\n2 3", and a table written
// one `[k]=v` per line came out as a single element holding the rest of the table. A newline
// inside quotes or a command substitution is still part of the element. The answers are
// bash 5.3's; busybox-w32 has no arrays.
func TestRuntime_arrayLiteralNewlinesSeparateElements(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{"a=(\n1\n'2 3'\n)\nprintf '[%s]' \"${a[@]}\"", "[1][2 3]"},
		{"a=(\n1 # one\n\"2 3\" # two\n)\nprintf '[%s]' \"${#a[@]}\" \"${a[@]}\"", "[2][1][2 3]"},
		{"f=([1]=a\n[3]=b\n)\necho \"${!f[@]}\" \"${f[3]}\"", "1 3 b\n"},
		{"declare -A m=(\n[k]=v\n[j]=w\n)\necho \"${m[j]}\" \"${#m[@]}\"", "w 2\n"},
		{"e=( $(printf 'p\\nq') \"r\ns\" )\nprintf '[%s]' \"${e[@]}\"", "[p][q][r\ns]"},
		{"a=(x)\na+=(\ny\nz\n)\necho \"${a[@]}\"", "x y z\n"},
		{"c=(\n#only\n)\necho \"${#c[@]}\"", "0\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}

// An operator among an array literal's elements is a syntax error, and so is a group that is
// not at the start of a command -- `a= (1 2)` is the assignment written with a stray blank.
// The operator was dropped and the literal made of the rest; the group ran as a command named
// __nemosh_group__. Both are refused before any of the script runs, with status 2. bash
// refuses them too, having run the lines before; busybox refuses every array literal. A
// quoted or escaped operator is an element like any other.
func TestRuntime_arrayLiteralRefusesOperators(t *testing.T) {
	for _, script := range []string{
		"echo before\na=(\n1\n&\n'2 3'\n)\necho after",
		"echo before\na=(1 | 2)\necho after",
		"echo before\na=(1 <2)\necho after",
		"echo before\na=(1 && 2)\necho after",
		"echo before\na=(1 ; 2)\necho after",
		"echo before\na= (1 '2 3')\necho after",
		"echo before\nx=1 (echo sub)\necho after",
	} {
		t.Run(script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(script); stdout != "" || status != 2 {
				t.Errorf("got %q/%d, want the script refused with status 2", stdout, status)
			}
		})
	}
	script := "a=( '&' \"|\" \\; x\\<y $(echo 'p|q') )\nprintf '[%s]' \"${a[@]}\""
	if stdout, status := runScriptCapturing(script); stdout != "[&][|][;][x<y][p|q]" || status != 0 {
		t.Errorf("got %q/%d, want each quoted operator kept as an element", stdout, status)
	}
}
