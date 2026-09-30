package runtime_test

import "testing"

// A negated compound may follow `&&`, `||` or `&`, as busybox and bash read it: `true && ! if
// false; then :; fi` is 1. It was "unexpected then", since `!` was looked for only where a line
// begins. After a pipe it is a syntax error, as busybox has it.
func TestRuntime_aNegatedCompoundMayFollowAnOperator(t *testing.T) {
	for _, test := range []struct{ script, want string }{
		{"true && ! if false; then :; fi; echo $?\n", "1\n"},
		{"false || ! while false; do :; done; echo $?\n", "1\n"},
		{"true && ! case x in y) ;; esac; echo $?\n", "1\n"},
		{"false && ! if true; then echo no; fi; echo $?\n", "1\n"},
		{"if true; then :; fi && ! if false; then :; fi; echo $?\n", "1\n"},
		{"for i in 1; do :; done && ! for j in 2; do false; done; echo $?\n", "0\n"},
		{"true && ! if true; then echo in; fi | cat; echo $?\n", "in\n1\n"},
		{"true & ! if true; then echo bg; fi; wait; echo $?\n", "bg\n0\n"},
		{"! true && ! if true; then :; fi; echo $?\n", "1\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox answers", stdout, status, test.want)
			}
		})
	}
	if stdout, status := runScriptCapturing("true | ! if true; then cat; fi\n"); stdout != "" || status != 2 {
		t.Errorf("true | ! if: got %q/%d, want it refused, as busybox refuses it", stdout, status)
	}
}
