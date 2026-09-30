package runtime_test

import (
	"os"
	"path/filepath"
	"testing"
)

// An error in a builtin that is not special ends that builtin and not the script, as busybox's
// evalcommand has it and bash goes on too; `command` makes a special builtin a plain one. Each
// of these ended the script. busybox's ash_test readonly1.
func TestRuntime_anErrorInAPlainBuiltinEndsOnlyThatBuiltin(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source.sh")
	if err := os.WriteFile(source, []byte("echo src-in\nr=4\necho src-after\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ script, want string }{
		{"readonly r=1\ncommand export r=2\necho \"st=$?\"\n", "st=2\n"},
		{"readonly r=1\ncommand unset r\necho \"st=$?\"\n", "st=2\n"},
		{"readonly r=1\ncommand readonly r=3\necho \"st=$?\"\n", "st=2\n"},
		{"readonly r=1\ncommand eval r=2\necho \"st=$?\"\n", "st=2\n"},
		{"readonly r=1\ncommand eval 'echo in; r=2; echo out'\necho \"st=$?\"\n", "in\nst=2\n"},
		{"readonly r=1\ncommand . '" + source + "'\necho \"st=$?\"\n", "src-in\nst=2\n"},
		{"set -u\ncommand eval 'echo $nope'\necho \"st=$?\"\n", "st=2\n"},
		{"command eval 'echo hi' > /nonexistent-dir/x\necho \"st=$?\"\n", "st=1\n"},
		{"readonly r=1\nlet r=5\necho \"st=$?\"\n", "st=2\n"},
		{"readonly r=1\ndeclare r=2\necho \"st=$?\"\n", "st=1\n"},
		{"readonly r=1\ntypeset r=2\necho \"st=$?\"\n", "st=1\n"},
		{"readonly r=1\n(eval r=2; echo BUG)\necho \"Fail:$?\"\n", "Fail:2\n"},
	} {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox answers", stdout, status, test.want)
			}
		})
	}
	// A special builtin's error, one in a function and an exit under command still end the
	// script, as in busybox.
	for _, script := range []string{
		"readonly r=1\nexport r=2\necho reached\n",
		"readonly r=1\neval r=2\necho reached\n",
		"readonly r=1\nf() { r=2; echo in; }\nf\necho reached\n",
		"readonly r=1\nf() { local r=2; echo in; }\nf\necho reached\n",
		"eval 'echo hi' > /nonexistent-dir/x\necho reached\n",
		"command eval 'exit 3'\necho reached\n",
	} {
		t.Run(script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(script); stdout != "" || status == 0 {
				t.Errorf("got %q/%d, want the script ended, as busybox ends it", stdout, status)
			}
		})
	}
}
