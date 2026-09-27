package runtime_test

import "testing"

// `${a[@]@K}` is an array as the pairs an array literal reads back, one word; `${a[@]@k}` is
// the same pairs as a word each, unquoted. Over anything that is not an array they quote as
// @Q does. Both were refused by name. The answers are bash 5.3's; busybox has neither.
func TestRuntime_keyValueTransforms(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`arr=(a "b c"); printf "[%s]" "${arr[@]@K}" ${arr[@]@K}; echo`, "[0 \"a\" 1 \"b c\"][0][\"a\"][1][\"b][c\"]\n"},
		{`arr=(a "b c"); printf "[%s]" "${arr[@]@k}"; printf "<%s>" "${arr[*]@k}"; echo`, "[0][a][1][b c]<0 a 1 b c>\n"},
		{`sp=([3]=x [7]=y); printf "[%s]" "${sp[@]@K}"; echo`, "[3 \"x\" 7 \"y\"]\n"},
		{`declare -A m=([k1]=v1); m["k 2"]=$'v\n2'; printf "[%s]" "${m[@]@K}"; echo`, "[k1 \"v1\" \"k 2\" $'v\\n2' ]\n"},
		{`declare -A m=([k1]=v1); m["k 2"]=w; printf "[%s]" "${m[@]@k}"; echo`, "[k1][v1][k 2][w]\n"},
		{`e=(); set -- "${e[@]@K}"; echo $#; set -- "${e[*]@K}"; echo $#`, "0\n1\n"},
		{`s="p q"; arr=(a "b c"); printf "[%s]" "${s@K}" "${s@k}" "${arr[1]@K}" "${arr@k}"; echo`, "['p q']['p q']['b c']['a']\n"},
		{`set -- x "y z"; printf "[%s]" "${@@K}" "${*@k}"; echo`, "['x']['y z']['x' 'y z']\n"},
		{`x=x; empty=; echo ${x@K} ${empty@K} ${undef@K} ${x@k}`, "'x' '' 'x'\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}
