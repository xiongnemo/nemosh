package applets

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"syscall"
)

// Applet diagnostics name the operand exactly as the user wrote it. The
// resolved host path names one machine and Windows spells its own errors, so
// neither belongs in text a script greps or a behavior case compares byte for
// byte (docs/design/windows-execution-model.md, "Error Diagnostics").
//
// The shell prints "<applet>: <error>", so none of these carry the applet name.
//
// busybox uses three shapes and is not consistent about which, so neither are
// we. Quoted behind a verb: open_or_warn (libbb/xfuncs_printf.c:169),
// remove_file (libbb/remove_file.c:23), bb_make_directory
// (libbb/make_directory.c:150). Quoted with no verb: rmdir, which says its
// format matches GNU's (coreutils/rmdir.c:78). Bare: everything reaching
// bb_simple_perror_msg, such as fopen_or_warn (libbb/wfopen.c:11), touch
// (coreutils/touch.c:188), chmod (coreutils/chmod.c:102) and the recursive walk
// find is built on (libbb/recursive_action.c:158).

type quotedError struct {
	action  string
	operand string
	err     error
}

func cannotOpen(operand string, err error) error {
	return quotedError{action: "open", operand: operand, err: err}
}

func cannotRemove(operand string, err error) error {
	return quotedError{action: "remove", operand: operand, err: err}
}

func cannotCreateDirectory(operand string, err error) error {
	return quotedError{action: "create directory", operand: operand, err: err}
}

func cannotCreate(operand string, err error) error {
	return quotedError{action: "create", operand: operand, err: err}
}

func cannotStat(operand string, err error) error {
	return quotedError{action: "stat", operand: operand, err: err}
}

func cannotRename(operand string, err error) error {
	return quotedError{action: "rename", operand: operand, err: err}
}

// quotedFailure is the verbless form GNU rmdir uses.
func quotedFailure(operand string, err error) error {
	return quotedError{operand: operand, err: err}
}

func (e quotedError) Error() string {
	if e.action == "" {
		return fmt.Sprintf("'%s': %s", e.operand, causeText(e.err))
	}
	return fmt.Sprintf("can't %s '%s': %s", e.action, e.operand, causeText(e.err))
}

func (e quotedError) Unwrap() error { return e.err }

type operandError struct {
	operand string
	err     error
}

func operandFailure(operand string, err error) error {
	return operandError{operand: operand, err: err}
}

func (e operandError) Error() string {
	return fmt.Sprintf("%s: %s", e.operand, causeText(e.err))
}

func (e operandError) Unwrap() error { return e.err }

// env and xargs both launch a COMMAND, and both follow the SUSv3 table busybox
// cites: BB_EXECVP_or_die raises xfunc_error_retval to 127 for ENOENT and 126
// otherwise before it dies (libbb/executable.c:117-122), and xargs names the
// same two statuses (findutils/xargs.c:385-390). Nemosh dispatches registered
// applets only, so only the not-found branch is reachable, and the wording says
// "not found" rather than busybox's "can't execute '%s'" because no execvp ran.
func commandNotFound(name string) error {
	return ExitStatusMessage(127, operandFailure(name, errors.New("not found")))
}

// cut, sort and uniq assemble and print their own diagnostic instead of
// returning one, so a read failure has to carry the operand back out to the
// print site. Everything else returns a quotedError or an operandError and
// lets the shell prefix the applet name.
//
// The three do not agree on a shape either. cut and sort open through
// fopen_or_warn_stdin, whose bb_simple_perror_msg prints the operand bare
// (libbb/wfopen_input.c:16). uniq opens through xopen, which quotes it behind a
// verb (libbb/xfuncs_printf.c:151).
type inputError struct {
	operand string
	quoted  bool
	err     error
}

func inputFailure(operand string, err error) error {
	return inputError{operand: operand, err: err}
}

func quotedInputFailure(operand string, err error) error {
	return inputError{operand: operand, quoted: true, err: err}
}

func (e inputError) Error() string { return e.err.Error() }

func (e inputError) Unwrap() error { return e.err }

// A failure that never named an operand -- a read off stdin, say -- prints as
// "<applet>: <cause>" rather than growing an empty operand field.
func inputDiagnostic(applet string, err error) string {
	inputErr, ok := errors.AsType[inputError](err)
	if !ok {
		return fmt.Sprintf("%s: %s", applet, causeText(err))
	}
	if inputErr.quoted {
		return applet + ": " + cannotOpen(inputErr.operand, inputErr.err).Error()
	}
	return applet + ": " + operandFailure(inputErr.operand, inputErr.err).Error()
}

// causeText spells a cause the way strerror does. The fallback unwraps
// *fs.PathError rather than printing it, because its own Error() repeats the
// host path this whole file exists to keep out of diagnostics.
func causeText(err error) string {
	if text, ok := platformCauseText(err); ok {
		return text
	}
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "No such file or directory"
	case errors.Is(err, fs.ErrPermission):
		return "Permission denied"
	case errors.Is(err, fs.ErrExist):
		return "File exists"
	// Go spells this one "is a directory"; strerror capitalizes it.
	case errors.Is(err, syscall.EISDIR):
		return "Is a directory"
	}
	if pathErr, ok := errors.AsType[*fs.PathError](err); ok {
		return pathErr.Err.Error()
	}
	// os.Rename fails with a *os.LinkError, which names both host paths.
	if linkErr, ok := errors.AsType[*os.LinkError](err); ok {
		return linkErr.Err.Error()
	}
	return err.Error()
}
