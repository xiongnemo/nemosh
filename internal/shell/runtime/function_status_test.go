package runtime_test

import "testing"

// A function's body starts with `$?` as its caller had it, as in both references, so a bare
// `return` first thing returns it and an ERR trap's handler sees the status that fired it. It
// started at 0: `err() { echo $?; }; trap err ERR` said 0 for every failure.
func TestRuntime_aFunctionStartsWithItsCallersStatus(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{"f() { echo \"in f: $?\"; }; false; f", "in f: 1\n"},
		{"f() { echo \"in f: $?\"; }; (exit 7); f; echo \"after: $?\"", "in f: 7\nafter: 0\n"},
		{"f() { return; }; false; f; echo \"st=$?\"", "st=1\n"},
		{"err() { echo \"err [$@] $?\"; }; trap 'err x y' ERR; false; (exit 42); trap - ERR", "err [x y] 1\nerr [x y] 42\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as both references answer", stdout, status, test.want)
			}
		})
	}
}
