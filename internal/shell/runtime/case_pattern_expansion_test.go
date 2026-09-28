package runtime_test

import "testing"

// A `;;` or a `)` inside an arithmetic command or an expansion is no case pattern's: in a brace
// group, `for ((n=2;;n++))` in an arm read as the arm's end, and `$((1+2)))` as a pattern whose
// `)` came before its last one. Each script was refused. bash's answers, measured; busybox has
// no arithmetic command but agrees on the second.
func TestCasePattern_anArithmeticsParenthesesAreItsOwn(t *testing.T) {
	script := "{\ncase a in\na)\n  for ((n=2;;n++)); do\n    break\n  done\nesac\n}\necho $n\n" +
		"f() {\n\tcase 3 in\n\t$((1+2)))\n\t\techo three\n\t\t;;\n\tesac\n}\nf\n"
	want := "2\nthree\n"
	if stdout, status := runScriptCapturing(script); stdout != want || status != 0 {
		t.Fatalf("got %q/%d, want %q/0, as bash answers", stdout, status, want)
	}
}
