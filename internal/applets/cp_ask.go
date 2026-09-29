package applets

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
)

// unlinkAnswer is what became of a destination in the way of a copy.
type unlinkAnswer int

const (
	unlinked       unlinkAnswer = iota // removed, so the copy can be made
	unlinkDeclined                     // -i asked and was told no: no copy, and no failure
	unlinkFailed                       // it could not be removed, and that has been said
)

// askAndUnlink is libbb's ask_and_unlink, for a destination that could not be made because
// something is there. -i asks first, on stderr, and reads the answer from stdin. Then what is
// there is removed, -f or no -f, as busybox-w32 does (FEATURE_NON_POSIX_CP). cause is why the
// destination could not be made, which is what a failure names rather than the removal's error.
func (r *cpRun) askAndUnlink(dest pathOperand, cause error) unlinkAnswer {
	if r.flags.interactive {
		fmt.Fprintf(r.stderr, "cp: overwrite '%s'? ", dest.operand)
		if !askYes(r.stdin) {
			return unlinkDeclined
		}
	}
	if err := removeForOverwrite(dest.host); err != nil {
		if cause == nil || errors.Is(cause, fs.ErrExist) {
			cause = err
		}
		r.fail(cannotCreate(dest.operand, cause))
		return unlinkFailed
	}
	if r.flags.removeDest && r.flags.verbose {
		fmt.Fprintf(r.stdout, "removed '%s'\n", dest.operand)
	}
	return unlinked
}

// askYes reads one line of an answer and reports whether it began with y or Y, as busybox's
// bb_ask_y_confirmation does. It reads a byte at a time, so the next question finds the next
// line still there.
func askYes(stdin io.Reader) bool {
	if stdin == nil {
		return false
	}
	var first byte
	one := make([]byte, 1)
	for {
		if n, err := stdin.Read(one); n == 0 || err != nil && n == 0 {
			break
		}
		if one[0] == '\n' {
			break
		}
		if first == 0 && one[0] != ' ' && one[0] != '\t' {
			first = one[0] | 0x20
		}
	}
	return first == 'y'
}

// finish keeps what -p asks to keep and reports the copy under -v: busybox's
// preserve_mode_ugid_time and verb_and_exit. A copy whose times or mode cannot be kept is still
// a copy, so that is said and the status is left alone.
func (r *cpRun) finish(source, dest pathOperand, info os.FileInfo) {
	if r.flags.preserve {
		if err := os.Chtimes(dest.host, info.ModTime(), info.ModTime()); err != nil {
			fmt.Fprintf(r.stderr, "cp: cannot preserve times of '%s': %s\n", dest.operand, causeText(err))
		}
		r.preserveOwner(dest, info)
		if err := applyPermissions(dest.host, bitsOfFileMode(info.Mode()), info.IsDir()); err != nil {
			fmt.Fprintf(r.stderr, "cp: cannot preserve permissions of '%s': %s\n", dest.operand, causeText(err))
		}
	}
	r.report(source, dest)
}

// preserveOwner gives the copy its source's owner and group, where the platform has them.
func (r *cpRun) preserveOwner(dest pathOperand, info os.FileInfo) {
	if err := copyOwner(dest.host, info); err != nil {
		fmt.Fprintf(r.stderr, "cp: cannot preserve ownership of '%s': %s\n", dest.operand, causeText(err))
	}
}

// report is -v's line for a copy.
func (r *cpRun) report(source, dest pathOperand) {
	if r.flags.verbose {
		fmt.Fprintf(r.stdout, "'%s' -> '%s'\n", source.operand, dest.operand)
	}
}

// fail names a failure and makes cp's status 1. It answers false, for a copy not made.
func (r *cpRun) fail(err error) bool {
	r.failed = true
	fmt.Fprintf(r.stderr, "cp: %v\n", err)
	return false
}

// status is cp's answer: err if the command itself could not go on, and otherwise 1 if any
// copy failed.
func (r *cpRun) status(err error) error {
	if err != nil {
		return err
	}
	if r.failed {
		return ExitStatus(1)
	}
	return nil
}
