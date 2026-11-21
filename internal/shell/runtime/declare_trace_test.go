package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// `declare -t` is bash's trace attribute: a function that has it inherits the DEBUG and RETURN
// traps, as every function does under `set -T`, and keeps it when it is defined again; on a
// variable it means nothing and `declare -p` shows it. busybox has no declare. The output is
// bash 5.3's, measured 2026-11-21. It was "declare: -t: invalid option", status 2, at the top
// of bash-preexec, whose hooks set their DEBUG trap from traced functions.
func TestDeclare_theTraceAttribute(t *testing.T) {
	script := `f() { trap -p DEBUG; echo "in f"; }
g() { echo "in g"; }
trap ': dbg' DEBUG
declare -ft f
f
declare -F f g
declare -F | grep -E ' (f|g)$'
declare -tix x=1
declare -p x
declare -t y
declare -p y
declare +t x
declare -p x
f() { trap -p DEBUG; echo "f redefined"; }
f
declare -F f
declare -ft nosuch; echo "status $?"
trap - DEBUG
h() { trap 'echo ret-h' RETURN; }
k() { echo in-k; }
declare -ft k
h; k
`
	want := `trap -- ': dbg' DEBUG
in f
f
g
declare -ft f
declare -f g
declare -itx x="1"
declare -t y
declare -ix x="1"
trap -- ': dbg' DEBUG
f redefined
f
status 1
ret-h
in-k
ret-h
`
	var stdout bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: new(bytes.Buffer)})
	rt.RunScript(context.Background(), script)
	if stdout.String() != want {
		t.Fatalf("stdout\n%s\nwant\n%s", stdout.String(), want)
	}
}

// A background job that is a process has the shell's functions with their trace attribute,
// and its variables with theirs, as a subshell does.
func TestJobState_carriesTheTraceAttribute(t *testing.T) {
	ctx := context.Background()
	original := New(applets.DefaultRegistry, Streams{Stdout: new(bytes.Buffer), Stderr: new(bytes.Buffer)})
	if status := original.RunScript(ctx, "f() { :; }\ndeclare -ft f\ndeclare -t v=1\n"); status != 0 {
		t.Fatalf("setup exited %d", status)
	}
	job, err := ParseScript("true\n")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeJobState(encodeJobState(original.captureJobState(job.program[0])))
	if err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	restored := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: new(bytes.Buffer)})
	if _, err := restored.restoreJobState(ctx, decoded); err != nil {
		t.Fatalf("restore: %v", err)
	}
	restored.RunScript(ctx, "declare -F; declare -p v\n")
	if want := "declare -ft f\ndeclare -t v=\"1\"\n"; stdout.String() != want {
		t.Fatalf("the job has %q, want %q", stdout.String(), want)
	}
}
