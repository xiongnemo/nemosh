package runtime

import (
	"context"
	"io"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A call of a function that makes nothing local costs one allocation more than its body does:
// the frame FUNCNAME and caller read, the positional parameters and the local scope are one
// record, and the scope's map is made only when something is made local. They were four: the
// frame, the parameters, the scope and its map -- a third of what a turn of a loop that calls a
// function allocated.
func TestAllocations_aFunctionCallIsOneRecord(t *testing.T) {
	ctx := context.Background()
	rt := New(applets.DefaultRegistry, Streams{Stdout: io.Discard, Stderr: io.Discard})
	if status := rt.RunScript(ctx, "f(){ :; }\n"); status != 0 {
		t.Fatalf("defining f exited %d", status)
	}
	definition, found := rt.functions[functionName{value: "f"}]
	if !found {
		t.Fatal("f was not defined")
	}
	body := testing.AllocsPerRun(100, func() { rt.executeCommandNode(ctx, definition.body, 0) })
	call := testing.AllocsPerRun(100, func() { rt.callFunctionWith(ctx, definition, nil, 0, nil) })
	if call-body > 1 {
		t.Errorf("a call of f allocated %v and its body %v: the call's own is %v, want 1", call, body, call-body)
	}
}
