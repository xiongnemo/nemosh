package runtime_test

import "testing"

// A compound's closer may have `&` and more after it, as busybox and bash read it: `done &
// echo x` runs the loop in the background and then the echo, and `done &> log` is bash's
// redirection of both outputs. Each was "missing done", "case: expected pattern)" or
// "unexpected word after a redirection".
func TestRuntime_aCloserMayHaveAnAmpersandAndMoreAfterIt(t *testing.T) {
	for _, test := range []struct{ script, want string }{
		{"while false; do :; done & echo x; wait\n", "x\n"},
		{"if true; then echo a; fi & wait; echo b\n", "a\nb\n"},
		{"for i in 1; do echo in; done & wait && echo ok\n", "in\nok\n"},
		{"case x in x) echo c;; esac & wait\n", "c\n"},
		{"if true; then echo a; fi&wait\n", "a\n"},
		{"while false; do :; done < /dev/null & echo z; wait\n", "z\n"},
		{"case x in x) echo c;; esac > /dev/null & echo w; wait\n", "w\n"},
		{"for i in 1 2; do echo $i >&2; done &> /dev/null; echo after\n", "after\n"},
		{"if false; then :; elif true; then echo e; fi & wait; echo f\n", "e\nf\n"},
		{"while false; do :; done & echo x && while false; do :; done; echo y\n", "x\ny\n"},
		{"while false; do :; done & p=$!; wait; [ -n \"$p\" ] && echo pid\n", "pid\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox answers", stdout, status, test.want)
			}
		})
	}
}
