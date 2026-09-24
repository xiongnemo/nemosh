package runtime_test

import "testing"

// A background `&` ends a command wherever `;` would, reserved word after it or not:
// busybox-w32 and bash 5.3 run each of these. nemosh read the first three as "missing
// done", "unexpected ;" and "expected separator before }". `a &; b` is still a syntax
// error, in both references and here.
func TestBackgroundOperator_endsACommandBeforeAReservedWord(t *testing.T) {
	for _, test := range []struct{ name, script, want string }{
		{name: "before done", script: "for i in 1 2; do echo a& done; wait\n", want: "a\na\n"},
		{name: "before fi", script: "if true; then sleep 0 & fi; wait; echo ok\n", want: "ok\n"},
		{name: "before a case terminator", script: "case a in a) true & ;; esac; wait; echo ok\n", want: "ok\n"},
		{name: "before a closing brace", script: "{ echo g & }; wait\n", want: "g\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			status, stdout, stderr := runSetScript(t, test.script)
			if status != 0 || stdout != test.want {
				t.Fatalf("status %d stdout %q stderr %q, want %q", status, stdout, stderr, test.want)
			}
		})
	}
	if status, _, _ := runSetScript(t, "echo a &; echo b\n"); status != 2 {
		t.Fatalf("`echo a &; echo b` = %d, want the syntax error both references give", status)
	}
}
