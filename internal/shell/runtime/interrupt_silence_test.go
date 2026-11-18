package runtime

import (
	"bytes"
	"context"
	"errors"
	"syscall"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// An applet that the shell's interrupt ended says nothing, whatever error its ending made, as a
// busybox applet the signal ends says nothing: wget's connection closed under it came back as
// `use of closed network connection`, printed after the ^C. A signal sent to a job is the same.
// An error that is not the interrupt's is still reported.
func TestAppletStatus_saysNothingWhenASignalEndedIt(t *testing.T) {
	var stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stderr: &stderr})
	for _, cause := range []error{errShellInterrupt, consoleInterrupt{prompt: true}, jobSignal(syscall.SIGTERM)} {
		ctx, cancel := context.WithCancelCause(context.Background())
		cancel(cause)
		stderr.Reset()
		status := rt.appletStatus(ctx, "wget", errors.New("use of closed network connection"))
		if stderr.Len() != 0 || status != contextStatus(ctx) {
			t.Errorf("ended by %v: said %q, status %d; want nothing and %d", cause, stderr.String(), status, contextStatus(ctx))
		}
	}
	stderr.Reset()
	if status := rt.appletStatus(context.Background(), "wget", errors.New("server returned 404 Not Found")); status != 1 || stderr.Len() == 0 {
		t.Errorf("a failure of its own: said %q, status %d", stderr.String(), status)
	}
}
