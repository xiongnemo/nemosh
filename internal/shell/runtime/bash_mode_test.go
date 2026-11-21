package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// Bash mode is the shell's for its life: a new runtime is not in it, one run as bash is and
// says bash's version, and a subshell and a background job that is a process are in it as
// their shell is, since each runs part of the same script.
func TestBashMode_isTheShellsAndItsSubshellsAndJobs(t *testing.T) {
	plain := New(applets.DefaultRegistry, Streams{Stdout: new(bytes.Buffer), Stderr: new(bytes.Buffer)})
	if plain.BashMode() {
		t.Fatal("a new runtime is in bash mode")
	}
	rt := New(applets.DefaultRegistry, Streams{Stdout: new(bytes.Buffer), Stderr: new(bytes.Buffer)})
	rt.EnterBashMode()
	if !rt.BashMode() {
		t.Fatal("EnterBashMode left the runtime out of bash mode")
	}
	if version, _ := rt.LookupVariable("BASH_VERSION"); version != "5.2.37(1)-release" {
		t.Errorf("BASH_VERSION is %q in bash mode", version)
	}
	subshell, err := rt.clone(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if !subshell.BashMode() {
		t.Error("a subshell of a shell in bash mode is out of it")
	}
	job, err := ParseScript("true\n")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeJobState(encodeJobState(rt.captureJobState(job.program[0])))
	if err != nil {
		t.Fatal(err)
	}
	restored := New(applets.DefaultRegistry, Streams{Stdout: new(bytes.Buffer), Stderr: new(bytes.Buffer)})
	if _, err := restored.restoreJobState(context.Background(), decoded); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if !restored.BashMode() {
		t.Error("a background job of a shell in bash mode is out of it")
	}
}
