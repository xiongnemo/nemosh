package runtime_test

import "testing"

// An array literal is an assignment's value, and a loop or case keyword with nothing after it
// is still that keyword. `echo a=(1 2)`, `for x in a=()` and `case a=() in` used the literal's
// text as a word; `for i.j in` looped with i.j as its variable; and a bare `for` ran as a
// command. busybox-w32 refuses all five before running anything, as this now does, with
// status 2; bash agrees except for `for i.j`, which it runs.
func TestRuntime_misplacedArrayLiteralIsASyntaxError(t *testing.T) {
	for _, script := range []string{
		"echo hi; for\necho status=$?",
		"echo hi; select\necho status=$?",
		"for i.j in a b c; do\n  echo hi\ndone\necho done",
		"f() {\n  for x in a=(); do\n    echo x=$x\n  done\n  echo done\n}\nf",
		"case a=() in\n  *) echo match;;\nesac",
		"echo hi\necho a=(1 2)",
	} {
		t.Run(script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(script); stdout != "" || status != 2 {
				t.Errorf("got %q/%d, want the script refused with status 2", stdout, status)
			}
		})
	}
	// Unchanged: a literal as an assignment and after a declaration utility, and a loop over
	// ordinary words.
	script := "a=(1 2); declare b=(3); f() { local c=(4); echo ${c[0]}; }; f; echo ${a[1]} ${b[0]}\nfor x in p q; do echo $x; done"
	if stdout, status := runScriptCapturing(script); stdout != "4\n2 3\np\nq\n" || status != 0 {
		t.Errorf("got %q/%d, want %q/0", stdout, status, "4\n2 3\np\nq\n")
	}
}
