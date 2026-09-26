package runtime_test

import "testing"

// `${a[@]}` is a field per element, as `$@` is a field per parameter, and the same rules
// follow from that. An empty array is no field at all, quoted or not, so a loop over one
// runs no times; here it was one empty field, and `for i in "${a[@]}"` ran once. Unquoted,
// each element is split by IFS and an empty one vanishes; here each element was kept
// whole. The answers are bash 5.3's, since busybox-w32 has no arrays.
func TestRuntime_arrayElementsExpandAsParametersDo(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`a=(); set -- "${a[@]}"; echo $#`, "0\n"},
		{`a=(); set -- ${a[@]}; echo $#`, "0\n"},
		{`a=(); for i in "${a[@]}"; do echo it; done; echo done`, "done\n"},
		{`declare -A m; set -- "${m[@]}"; echo $#`, "0\n"},
		{`a=(); echo "<${a[@]}>"`, "<>\n"},
		{`a=(); set -- x"${a[@]}"y; echo $# "$1"`, "1 xy\n"},
		{`a=(1 "2 3"); set -- ${a[@]}; echo $#`, "3\n"},
		{`a=(1 "2 3"); set -- "${a[@]}"; echo $#`, "2\n"},
		{`a=("" x); set -- ${a[@]}; echo $#`, "1\n"},
		{`a=("" x); set -- "${a[@]}"; echo $#`, "2\n"},
		{`IFS=:; a=('a:b' c); set -- ${a[@]}; echo $#`, "3\n"},
		{`a=(x y); set -- p"${a[@]}"q; echo $# "$1" "$2"`, "2 px yq\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}
