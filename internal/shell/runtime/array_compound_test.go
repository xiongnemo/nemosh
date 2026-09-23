package runtime_test

import "testing"

// Compound assignment with subscripts, for both kinds of array. `[k]=v` used to be an
// ordinary word -- stored as the text `[k]=v` in an indexed array, and dropped entirely by
// `declare -A m=(...)`, leaving an empty map. Every answer here is bash's, measured.
func TestCompoundAssignment_honoursSubscripts(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
	}{
		{name: "a lookup table", script: "declare -A m=([a]=1 [b]=2)\necho \"${m[b]}\"\n", want: "2\n"},
		{name: "assigned after declaring", script: "declare -A m\nm=([a]=1 [b]=\"x y\")\necho \"${m[b]}|${#m[@]}\"\n", want: "x y|2\n"},
		{name: "typeset is the same", script: "typeset -A t=([x]=1)\necho \"${t[x]}\"\n", want: "1\n"},
		{name: "keys expand", script: "k=kk\ndeclare -A m=([$k]=v [q]=$(echo z))\necho \"${m[kk]}${m[q]}\"\n", want: "vz\n"},
		{name: "appending keys", script: "declare -A m=([a]=1)\nm+=([c]=3)\necho \"${m[a]}${m[c]} ${#m[@]}\"\n", want: "13 2\n"},
		{name: "reassigning replaces", script: "declare -A m=([k]=v)\nm=([j]=w)\necho \"${!m[@]}\"\n", want: "j\n"},
		{name: "unsubscripted words pair up", script: "declare -A m=(k1 v1 k2 v2)\necho \"${m[k2]}\"\n", want: "v2\n"},
		{name: "indexed, out of order", script: "a=([2]=two [0]=zero)\necho \"${a[@]} ${!a[@]}\"\n", want: "zero two 0 2\n"},
		{name: "an unsubscripted word follows", script: "a=([5]=x y)\necho \"${!a[@]}\"\n", want: "5 6\n"},
		{name: "a subscript is arithmetic", script: "i=1\na=([i+1]=v)\necho \"${!a[@]}\"\n", want: "2\n"},
		{name: "mixed with plain words", script: "x=(one \"two words\" [3]=three)\necho \"${#x[@]} ${x[1]} ${x[3]}\"\n", want: "3 two words three\n"},
		{name: "appending indices", script: "a=(x y)\na+=([5]=f g)\necho \"${!a[@]}: ${a[@]}\"\n", want: "0 1 5 6: x y f g\n"},
		{name: "a quoted bracket is a value", script: "a=(\"[k]=v\")\necho \"${a[0]}\"\n", want: "[k]=v\n"},
		// declare -p prints only the indices that are set; it printed every slot, so a
		// gap came out as `[1]=""` and an unset element came back with its old value.
		{name: "declare -p over a gap", script: "a=(x y z)\nunset 'a[1]'\ndeclare -p a\n", want: "declare -a a=([0]=\"x\" [2]=\"z\")\n"},
		{name: "declare -p of a map", script: "declare -A m=([k]=v)\ndeclare -p m\n", want: "declare -A m=([k]=\"v\" )\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if stdout, _ := runScriptCapturing(test.script); stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}

// A readonly array refuses a new list and a new element alike, as a shell error. Neither
// path checked, so both wrote through.
func TestCompoundAssignment_refusesAReadonlyArray(t *testing.T) {
	for _, script := range []string{
		"readonly a=(1)\na=(2)\necho reached\n",
		"a=(1 2)\nreadonly a\na[0]=9\necho reached\n",
	} {
		if stdout, status := runScriptCapturing(script); stdout != "" || status != 2 {
			t.Fatalf("%q: got %q/%d, want the script ended with status 2", script, stdout, status)
		}
	}
}
