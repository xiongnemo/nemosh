package runtime_test

import "testing"

// `unset a[i]`, and the two things it used to leave behind.
//
// It returned 0 and did nothing: the subscript was never parsed, so the whole `a[1]` was
// looked up as a variable name, not found, and deleting a name that is not there succeeds.
// A script that removed an element and carried on was silently wrong -- the failure mode
// AGENTS.md singles out, where an absent capability has to be loud.
//
// Every expectation here was measured against bash on the same input rather than reasoned
// out, because the interesting part is what happens to the *other* indices and bash is the
// only authority on that.

func TestUnset_removesOneElementAndLeavesAGap(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
	}{
		// The gap is the point. Compacting would shift `z` from index 2 to index 1 and
		// silently change what every later read means.
		{name: "middle", script: `a=(x y z); unset "a[1]"; echo "${a[@]}"`, want: "x z\n"},
		{name: "indices survive", script: `a=(x y z); unset "a[1]"; echo "${!a[@]}"`, want: "0 2\n"},
		{name: "later index unmoved", script: `a=(x y z); unset "a[1]"; echo "${a[2]}"`, want: "z\n"},
		{name: "count drops", script: `a=(x y z); unset "a[1]"; echo "${#a[@]}"`, want: "2\n"},
		{name: "the hole reads empty", script: `a=(x y z); unset "a[1]"; echo "${a[1]:-gone}"`, want: "gone\n"},
		{name: "first", script: `a=(x y z); unset "a[0]"; echo "${!a[@]}"`, want: "1 2\n"},

		// A subscript is arithmetic, so a variable works without a dollar -- the same
		// rule `${a[i]}` follows.
		{name: "variable subscript", script: `a=(x y z); i=2; unset "a[i]"; echo "${a[@]}"`, want: "x y\n"},
		{name: "expression subscript", script: `a=(x y z); unset "a[1+1]"; echo "${a[@]}"`, want: "x y\n"},

		// Out of range is not an error, which is what bash does.
		{name: "past the end", script: `a=(x y z); unset "a[9]"; echo "${a[@]} $?"`, want: "x y z 0\n"},
		{name: "no such array", script: `unset "nosuch[1]"; echo $?`, want: "0\n"},

		// Appending after a gap continues from the slice's length, not from the highest
		// live index -- so the new element lands at 3 and the hole at 1 stays a hole.
		{name: "append after a gap", script: `a=(x y z); unset "a[1]"; a+=(q); echo "${!a[@]}"`, want: "0 2 3\n"},

		// `[@]` and `[*]` name the whole array.
		{name: "all with at", script: `a=(x y z); unset "a[@]"; echo "${#a[@]}"`, want: "0\n"},
		{name: "all with star", script: `a=(x y z); unset "a[*]"; echo "${#a[@]}"`, want: "0\n"},

		// And the plain form, which never reached the array table at all: `unset a`
		// deleted the scalar of that name and left the array behind, so `a[5]=q`
		// afterwards appended to the old elements instead of starting fresh.
		{name: "whole array", script: `a=(x y z); unset a; a[5]=q; echo "${!a[@]}"`, want: "5\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			status, stdout, stderr := runSetScript(t, test.script)
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
			if stdout != test.want {
				t.Fatalf("%s\n  got  %q\n  want %q", test.script, stdout, test.want)
			}
		})
	}
}

// An associative name is answered by key, so its subscript is a word rather than
// arithmetic. `m[0]` is a perfectly good key and must not be read as an index.
func TestUnset_removesAnAssociativeKey(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
	}{
		{name: "by key", script: `declare -A m; m[k]=v; m[j]=w; unset "m[k]"; echo "${m[@]}"`, want: "w\n"},
		{name: "keys survive in order", script: `declare -A m; m[k]=v; m[j]=w; unset "m[k]"; echo "${!m[@]}"`, want: "j\n"},
		{name: "a numeric key is a key", script: `declare -A m; m[0]=zero; unset "m[0]"; echo "n=${#m[@]}"`, want: "n=0\n"},
		{name: "absent key is not an error", script: `declare -A m; m[k]=v; unset "m[nope]"; echo "${m[@]} $?"`, want: "v 0\n"},

		// The declaration goes with the value. It did not: the name stayed associative
		// and its keys intact, so a later `m[j]=w` added to the old map rather than
		// making a fresh indexed array.
		{name: "whole map", script: `declare -A m; m[k]=v; unset m; echo "n=${#m[@]}"`, want: "n=0\n"},
		{name: "and stops being associative", script: `declare -A m; m[k]=v; unset m; m[j]=w; echo "${!m[@]}"`, want: "0\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			status, stdout, stderr := runSetScript(t, test.script)
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
			if stdout != test.want {
				t.Fatalf("%s\n  got  %q\n  want %q", test.script, stdout, test.want)
			}
		})
	}
}
