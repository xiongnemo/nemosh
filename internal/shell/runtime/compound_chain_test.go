package runtime_test

import "testing"

// A compound may follow another across `|`, `&&` or `||`, on one line or where the first one
// closes: `done | while read x; do ...; done`, `fi && if ...`. The line that closed the first
// was read as a command called `done` or `fi`, so the first compound never closed and each
// script was refused. The `||` after an if whose condition failed runs nothing, as the if's
// status is 0. Both references print every line, measured.
func TestCompound_followsAnotherAcrossAnOperator(t *testing.T) {
	script := "for i in 1 2; do echo $i; done | while read x; do echo \"got $x\"; done\n" +
		"if true; then echo a; fi && if true; then echo b; fi\n" +
		"if false; then :; fi || while true; do echo w; break; done\n" +
		"case a in a) echo c;; esac && if true; then echo d; fi\n" +
		"for i in 1; do\n echo f$i\ndone | while read x; do\n echo \"r $x\"\ndone\n" +
		"if true; then\n echo e\nfi &&\nif true; then\n echo g\nfi\n" +
		"if true; then\n echo h\nfi &&\necho i &&\ncase x in\nx) echo j;;\nesac\n" +
		"while false; do :; done &&\nfor i in 1; do\n echo k$i\ndone\n" +
		"for i in 1 2; do echo $i; done | while read x; do echo \"l$x\"; done | cat\n" +
		"if true; then echo m; fi 2>/dev/null |\nwhile read x; do echo \"n $x\"; done\n"
	want := "got 1\ngot 2\na\nb\nc\nd\nr f1\ne\ng\nh\ni\nj\nk1\nl1\nl2\nn m\n"
	if stdout, status := runScriptCapturing(script); stdout != want || status != 0 {
		t.Fatalf("got %q/%d, want %q/0, as both references answer", stdout, status, want)
	}
}
