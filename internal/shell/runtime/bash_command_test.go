package runtime_test

import "testing"

// $BASH_COMMAND is the simple command running, as it was written, and while a trap runs the
// command the trap came in on, as bash has it: an ERR trap's is the command that failed, and
// an EXIT trap's the last one run. busybox-w32 has none. It was unset, so the common
// `trap 'echo "failed: $BASH_COMMAND"' ERR` named nothing.
func TestRuntime_bashCommandIsTheRunningCommand(t *testing.T) {
	for index, test := range []struct{ script, want string }{
		{"trap 'echo \"failed: [$BASH_COMMAND]\"' ERR\nfalse x \"y z\"\necho after\n", "failed: [false x \"y z\"]\nafter\n"},
		{"echo \"[$BASH_COMMAND]\"\n", "[echo \"[$BASH_COMMAND]\"]\n"},
		{"echo  a   \"[$BASH_COMMAND]\"\n", "a [echo a \"[$BASH_COMMAND]\"]\n"},
		{"f() { echo \"[$BASH_COMMAND]\"; }; f a\n", "[echo \"[$BASH_COMMAND]\"]\n"},
		{"trap 'echo \"[$BASH_COMMAND]\"' EXIT\necho hi\n", "hi\n[echo hi]\n"},
		{"v=$(echo \"[$BASH_COMMAND]\"); echo \"$v\"\n", "[echo \"[$BASH_COMMAND]\"]\n"},
		// After a pipeline, its last simple command, in the shell, and that command's line.
		{"trap 'echo \"err $LINENO [$BASH_COMMAND]\"' ERR\necho x\ntrue | false y\ntrue | { false; }\necho end\n", "x\nerr 3 [false y]\nerr 4 [true]\nend\n"},
	} {
		if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
			t.Errorf("%d: %q: got %q/%d, want %q/0, as bash answers", index, test.script, stdout, status, test.want)
		}
	}
}
