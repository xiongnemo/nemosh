package runtime_test

import (
	"testing"
)

// What a function body ends with carries out of the call: `exit`, `break`, `exec`, a shell
// error. Only `return` stops there. The call used to answer only a status, so `exit` in a
// function did not exit -- `die boom; echo next` printed `next` with status 0 -- and it had
// been so since the typed runtime, untested. Every answer is busybox-w32's, measured.
func TestFunction_carriesItsControlOut(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		stdout string
		status int
	}{
		{name: "exit in a function exits", script: "f() { exit 3; }\nf\necho after\n", status: 3},
		{name: "the die idiom", script: "die() { echo \"fatal: $1\" >&2; exit 1; }\ndie boom\necho next\n", status: 1},
		{name: "exit from a nested call", script: "g() { exit 4; }\nf() { g; echo in-f; }\nf\necho after\n", status: 4},
		{name: "return stops at the function", script: "f() { return 4; echo no; }\nf\necho \"st=$?\"\n", stdout: "st=4\n"},
		// busybox's answer; bash refuses a break outside a loop in the function's own body.
		{name: "break leaves the caller's loop", script: "f() { break; }\nfor i in 1 2; do f; echo $i; done\necho done\n", stdout: "done\n"},
		{name: "a shell error in a function aborts", script: "set -u\nf() { echo $nope; echo in-f; }\nf\necho after\n", status: 2},
		// Contained where a subshell would contain it.
		{name: "exit in a subshell call", script: "f() { exit 6; }\n(f)\necho \"st=$?\"\n", stdout: "st=6\n"},
		{name: "exit in a substitution", script: "f() { exit 7; }\nx=$(f)\necho \"st=$?\"\n", stdout: "st=7\n"},
		{name: "exit in a pipeline stage", script: "f() { exit 5; }\nf | cat\necho \"st=$?\"\n", stdout: "st=0\n"},
		// A prefix assignment is the function's for the call, and restored after it even
		// when the body assigned the name itself; anything else the body set stays.
		{name: "prefix assignment is restored", script: "x=1\nf() { echo \"in=$x\"; x=inner; y=set; }\nx=2 f\necho \"x=$x y=$y\"\n", stdout: "in=2\nx=1 y=set\n"},
		{name: "redirections apply to the call", script: "f() { echo out; echo err >&2; }\nf >/dev/null 2>&1\necho quiet\n", stdout: "quiet\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.stdout || status != test.status {
				t.Fatalf("got %q/%d, want %q/%d", stdout, status, test.stdout, test.status)
			}
		})
	}
}

// An assignment to a readonly variable is a shell error, which ends a script with status 2
// wherever busybox-w32 ends it -- and a subshell ends only itself, and `read` only refuses.
// These used to answer 1 and carry on, and several of them wrote the variable anyway: the
// for loop, `${R:=x}` and arithmetic each wrote the map themselves.
func TestReadonly_assignmentIsAShellError(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		stdout string
		status int
	}{
		{name: "an assignment", script: "readonly R=1\nR=2\necho reached\n", status: 2},
		{name: "a prefix assignment", script: "readonly R=1\nR=2 true\necho reached\n", status: 2},
		{name: "export", script: "readonly R=1\nexport R=2\necho reached\n", status: 2},
		{name: "unset", script: "readonly R=1\nunset R\necho reached\n", status: 2},
		{name: "local", script: "readonly R=1\nf() { local R; }\nf\necho reached\n", status: 2},
		{name: "a for loop", script: "readonly R=1\nfor R in 9; do echo loop; done\necho reached\n", status: 2},
		{name: "arithmetic", script: "readonly R=1\n: $((R=5))\necho reached\n", status: 2},
		{name: "increment", script: "readonly R=1\n((R++))\necho reached\n", status: 2},
		{name: "an arithmetic for", script: "readonly R=1\nfor ((R=0; R<2; R++)); do :; done\necho reached\n", status: 2},
		{name: "assign-default", script: "readonly R\n: ${R:=7}\necho reached\n", status: 2},
		{name: "not caught by ||", script: "readonly R=1\nR=2 || echo handled\necho reached\n", status: 2},
		{name: "inside a function", script: "readonly R=1\nf() { R=2; echo in; }\nf\necho reached\n", status: 2},
		{name: "a subshell ends only itself", script: "readonly R=1\n(R=2)\necho \"reached $?\"\n", stdout: "reached 2\n"},
		{name: "a substitution ends only itself", script: "readonly R=1\nx=$(R=2; echo sub)\necho \"reached [$x]\"\n", stdout: "reached []\n"},
		{name: "read refuses and goes on", script: "readonly R=1\nread R <<< x\necho \"reached $? R=$R\"\n", stdout: "reached 2 R=1\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.stdout || status != test.status {
				t.Fatalf("got %q/%d, want %q/%d", stdout, status, test.stdout, test.status)
			}
		})
	}
}

// Every write goes through the one assignment path, so an exported name reaches the
// environment a child sees however it was assigned. Three of these used to leave the child
// with the old value.
func TestAssignment_reachesTheEnvironmentWhateverWroteIt(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
	}{
		{name: "arithmetic", script: "export x=0\n: $((x=5))\nenv | grep '^x='\n"},
		{name: "increment", script: "export x=4\n((x++))\nenv | grep '^x='\n"},
		{name: "a for loop", script: "export x=0\nfor x in 5; do env | grep '^x='; done\n"},
		{name: "assign-default", script: "export x\n: ${x:=5}\nenv | grep '^x='\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if stdout, _ := runScriptCapturing(test.script); stdout != "x=5\n" {
				t.Fatalf("the child saw %q, want x=5", stdout)
			}
		})
	}
}
