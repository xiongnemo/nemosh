package runtime_test

import "testing"

// A special builtin's usage error ends a script, as busybox raises it: a letter set has not
// got, a count or a status that is no number, an option unset has not got, and a bad option
// to local, which is special there. Each answered 2 and the script went on. A -o name set has
// not got is 1 and the script goes on, as busybox's minus_o answers it; under command, and in
// a builtin that is not special, the error ends only the builtin.
func TestRuntime_aSpecialBuiltinsUsageErrorEndsTheScript(t *testing.T) {
	for _, script := range []string{
		"set -Q\necho reached\n",
		"shift x\necho reached\n",
		"unset -Q x\necho reached\n",
		"f() { return x; }\nf\necho reached\n",
		"exit x\necho reached\n",
		"f() { local -Q x; }\nf\necho reached\n",
	} {
		t.Run(script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(script); stdout != "" || status != 2 {
				t.Errorf("got %q/%d, want the script ended with 2, as busybox ends it", stdout, status)
			}
		})
	}
	for _, test := range []struct{ script, want string }{
		{"set -o nosuchopt\necho \"st=$?\"\n", "st=1\n"},
		{"set +o nosuchopt\necho \"st=$?\"\n", "st=1\n"},
		{"command set -Q\necho \"st=$?\"\n", "st=2\n"},
		{"declare -Q x\necho \"st=$?\"\n", "st=2\n"},
		{"f() { return 3; }\nf\necho \"st=$?\"\n", "st=3\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox answers", stdout, status, test.want)
			}
		})
	}
}
