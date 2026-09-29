package runtime

import (
	"fmt"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// redirectFailure is a redirection that could not be made, worded as busybox's openredirect
// words it (shell/ash.c, sh_open and errmsg): `cannot create FILE` for one that writes, `cannot
// open FILE` for `<`, and for a file or directory that is not there `nonexistent directory` and
// `no such file`. It was `redirect descriptor 1: open x/y: open C:/work/x/y: The system cannot
// find the path specified.`, the host path and Windows' own sentence in it.
type redirectFailure struct {
	create bool
	path   string
	err    error
}

func (f redirectFailure) Error() string {
	action, missing := "open", "no such file"
	if f.create {
		action, missing = "create", "nonexistent directory"
	}
	cause := applets.CauseText(f.err)
	if cause == "No such file or directory" || cause == "Not a directory" {
		cause = missing
	}
	return fmt.Sprintf("cannot %s %s: %s", action, f.path, cause)
}

func (f redirectFailure) Unwrap() error { return f.err }

// dupFailure is `>&N` or `<&N` with N not open, which busybox's dup2_or_raise reports as the
// call that failed: `dup2(5,1): Bad file descriptor`.
type dupFailure struct {
	source, target int
	err            error
}

func (f dupFailure) Error() string {
	return fmt.Sprintf("dup2(%d,%d): %s", f.source, f.target, badDescriptor{}.Error())
}

func (f dupFailure) Unwrap() error { return f.err }

// badDescriptor is a descriptor the table does not hold, EBADF as dup2(2) calls it.
type badDescriptor struct{ err error }

func (e badDescriptor) Error() string { return "Bad file descriptor" }
func (e badDescriptor) Unwrap() error { return e.err }
