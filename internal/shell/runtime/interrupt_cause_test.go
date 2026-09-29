package runtime

import (
	"errors"
	"syscall"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// The shell's interrupt, ^C, and SIGINT sent to a job are both applets.ErrInterrupt, so an
// applet told to ignore SIGINT, tee -i, can tell them from the rest.
func TestInterruptCauses_areAppletsErrInterrupt(t *testing.T) {
	if !errors.Is(errShellInterrupt, applets.ErrInterrupt) {
		t.Error("the shell's interrupt is not applets.ErrInterrupt")
	}
	if !errors.Is(jobSignal(syscall.SIGINT), applets.ErrInterrupt) {
		t.Error("SIGINT to a job is not applets.ErrInterrupt")
	}
	if errors.Is(jobSignal(syscall.SIGTERM), applets.ErrInterrupt) {
		t.Error("SIGTERM to a job is applets.ErrInterrupt")
	}
}
