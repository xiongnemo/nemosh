package runtime_test

import "testing"

// A heredoc inside a `$(`, double-quoted or not, takes its body from the lines after its own,
// and one outside the `$(` from the lines after the one the `$(` closes on. `x="$(cat <<EOF`
// was refused as `<<` with no target, since the `"` hid the heredoc from the scan, and `cat
// <<EOF; x=$(` took its body from inside the substitution. Both references print every line
// here, measured.
func TestHeredoc_insideASubstitutionAndAroundOne(t *testing.T) {
	script := "x=\"$(cat <<EOF\nhi \"there\" (x) ; y\nEOF\n)\"; echo \"[$x]\"\n" +
		"y=\"$(cat <<-'END'\n\tliteral $HOME\n\tEND\n)\"\necho \"[$y]\"\n" +
		"z=\"pre $(cat <<EOF\none\nEOF\n) post\"; echo \"[$z]\"\n" +
		"w=\"$(cat <<A; cat <<B\na\nA\nb\nB\n)\"; echo \"[$w]\"\n" +
		"v=$(cat <<EOF\nunquoted\nEOF\n); echo \"[$v]\"\n" +
		"cat <<EOF; u=\"$(\necho hi\n)\"\nbody\nEOF\necho \"[$u]\"\n" +
		"cat <<EOF; t=$(\necho ho\n)\ntbody\nEOF\necho \"[$t]\"\n" +
		"cat <<A; s=$(cat <<B\nb\nB\n)\na\nA\necho \"[$s]\"\n"
	want := "[hi \"there\" (x) ; y]\n[literal $HOME]\n[pre one post]\n[a\nb]\n[unquoted]\n" +
		"body\n[hi]\ntbody\n[ho]\na\n[b]\n"
	if stdout, status := runScriptCapturing(script); stdout != want || status != 0 {
		t.Fatalf("got %q/%d, want %q/0, as both references answer", stdout, status, want)
	}
}
