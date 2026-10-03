package runtime

import (
	"errors"
	"fmt"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// AppletFailure turns an applet's error into the status and the one-line
// diagnostic that go with it. Exported because `nemosh cat missing` has to fail
// exactly the way `cat missing` inside the shell does, and the CLI cannot reach
// the shell's copy of this. It used to have no copy at all: direct dispatch
// dropped the applet-name prefix, and printed nothing whatever when the failure
// carried its own status, so `nemosh env python3` exited 127 in silence.
//
// An empty message means there is nothing to print: a bare applets.ExitStatus
// carries a status without a diagnostic, and so does ErrExitFalse.
func AppletFailure(name string, err error) (int, string) {
	if err == nil {
		return 0, ""
	}
	// A reader that went away ends the output, quietly and with SIGPIPE's status, as busybox's
	// applets end: 141, which is what `busybox seq 1 1000000 | head -1` leaves. Here rather than
	// at the two call sites, because the whole point of this function is that a direct
	// invocation and the same command inside the shell answer identically.
	if isClosedPipeError(err) {
		return brokenPipeStatus, ""
	}
	if status, ok := applets.StatusCode(err); ok {
		if message, ok := applets.StatusMessage(err); ok {
			return status, name + ": " + message
		}
		return status, ""
	}
	if errors.Is(err, applets.ErrExitFalse) {
		return 1, ""
	}
	return 1, fmt.Sprintf("%s: %v", name, err)
}
