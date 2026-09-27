package runtime_test

import "testing"

// An associative array's key is its subscript expanded as a word, as bash expands it: a
// positional parameter, a quoted parameter, a command substitution, text around a parameter.
// Only a bare `$name` was expanded, so `${m[$1]}` read the empty key and `m["$k"]=v` wrote
// the key $k. busybox has no arrays.
func TestAssociative_keyIsExpandedAsAWord(t *testing.T) {
	// When
	status, stdout, stderr := runSetScript(t, "declare -A m=([sunday]=jam [\"a b\"]=x [k-1]=y)\n"+
		"k=sunday\n"+
		"f() { echo \"1:${m[$1]} 2:${m[\"$1\"]}\"; }\nf sunday\n"+
		"echo \"3:${m[\"$k\"]} 4:${m[a b]} 5:${m[$(echo sunday)]} 6:${m[k-$((0+1))]}\"\n"+
		"m[\"$k-2\"]=z\necho \"7:${m[sunday-2]}\"\n")

	// Then
	want := "1:jam 2:jam\n3:jam 4:x 5:jam 6:y\n7:z\n"
	if stdout != want || status != 0 {
		t.Fatalf("got %q/%d, want %q/0; stderr = %q", stdout, status, want, stderr)
	}
}
