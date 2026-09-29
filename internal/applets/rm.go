package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// rm is busybox's (coreutils/rm.c): `rm [-fiRrv] FILE...`, each FILE removed as libbb's
// remove_file removes it (libbb/remove_file.c). -R and -r remove a directory and what it holds,
// -v says `removed 'x'` and `removed directory: 'x'`, -i asks before each, and -f says nothing
// of a FILE that is not there; of -f and -i the later wins. A FILE whose last component is .
// or .. is refused.
//
// That refusal was missing, so `rm -rf ..` removed everything in the parent directory, where
// busybox answers `can't remove '.' or '..'` and touches nothing. -R, -i and -v were refused as
// invalid options.
//
// What cannot be written is asked about first, as busybox asks, when stdin is a terminal and
// -f was not given; otherwise it is removed, a read-only file too, as busybox-w32 removes one.
// A directory's contents are all tried even after one fails, and the directory is removed only
// if every one of them was, so one file in use yields one line and not a line per directory
// above it.
type rmRun struct {
	// applet names it in messages, rm unless mv is removing what it copied across volumes.
	applet                                 string
	force, interactive, recursive, verbose bool
	// terminal is whether stdin is one, the condition for asking about what cannot be written.
	terminal       bool
	stdin          io.Reader
	stdout, stderr io.Writer
	failed         bool
}

func newRmApplet() Applet {
	return simpleApplet{name: "rm", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		options, operands, err := parseAppletOptions(ctx, args, "fiRrv", "")
		if err != nil {
			return err
		}
		last := options.last("fi")
		run := &rmRun{force: last == 'f', interactive: last == 'i', recursive: options.has('R') || options.has('r'),
			verbose: options.has('v'), terminal: inputIsTerminal(ctx, stdin), stdin: stdin, stdout: stdout, stderr: stderr}
		if len(operands) == 0 {
			// -f makes no operand at all acceptable, which is what `rm -f $unset` in a cleanup
			// script relies on.
			if run.force {
				return nil
			}
			return missingOperand()
		}
		view := ProcessViewFromContext(ctx)
		for _, operand := range operands {
			if err := ctx.Err(); err != nil {
				return err
			}
			if base := lastPathComponent(operand); base == "." || base == ".." {
				run.fail(errors.New("cannot remove '.' or '..'"))
				continue
			}
			native, err := resolveHostPath(view, operand)
			if err != nil {
				run.fail(err)
				continue
			}
			run.remove(native, operand)
		}
		if run.failed {
			return ExitStatus(1)
		}
		return nil
	}}
}

// remove is libbb's remove_file: it removes native, shown as display, and everything under it,
// and answers whether it did or was told not to.
func (r *rmRun) remove(native, display string) bool {
	// Lstat, so a symbolic link is removed and never followed: `rm -rf link-to-home` removes the
	// link.
	info, err := os.Lstat(native)
	if err != nil {
		if !isNotThere(err) {
			return r.fail(cannotStat(display, err))
		}
		if !r.force {
			return r.fail(cannotRemove(display, err))
		}
		return true
	}
	if !info.IsDir() {
		if r.interactive || !r.force && r.terminal && info.Mode()&os.ModeSymlink == 0 && !canWrite(native, info) {
			if !r.ask("remove '%s'? ", display) {
				return true
			}
		}
		if err := removeForOverwrite(native); err != nil {
			return r.fail(cannotRemove(display, err))
		}
		if r.verbose {
			fmt.Fprintf(r.stdout, "removed '%s'\n", display)
		}
		return true
	}
	// Without -r a directory is refused whether or not it is empty, and -f does not excuse it:
	// os.Remove would take an empty one, which made this shell more destructive than busybox.
	if !r.recursive {
		return r.fail(fmt.Errorf("'%s' is a directory", display))
	}
	if r.interactive || !r.force && r.terminal && !canWrite(native, info) {
		if !r.ask("descend into directory '%s'? ", display) {
			return true
		}
	}
	entries, err := os.ReadDir(native)
	if err != nil {
		return r.fail(cannotRemove(display, err))
	}
	removed := true
	for _, entry := range entries {
		removed = r.remove(filepath.Join(native, entry.Name()), joinOperand(display, entry.Name())) && removed
	}
	if r.interactive && !r.ask("remove directory '%s'? ", display) {
		return removed
	}
	if !removed {
		// Still here because of something already said above.
		return false
	}
	if err := removeDirectory(native); err != nil {
		return r.fail(cannotRemove(display, err))
	}
	if r.verbose {
		fmt.Fprintf(r.stdout, "removed directory: '%s'\n", display)
	}
	return true
}

// ask puts a question on stderr and reads the answer from stdin.
func (r *rmRun) ask(question, display string) bool {
	fmt.Fprintf(r.stderr, r.name()+": "+question, display)
	return askYes(r.stdin)
}

// fail names a failure and makes rm's status 1. It answers false, for a file not removed.
func (r *rmRun) fail(err error) bool {
	r.failed = true
	fmt.Fprintf(r.stderr, "%s: %v\n", r.name(), err)
	return false
}

// name is the applet the messages are from.
func (r *rmRun) name() string {
	if r.applet == "" {
		return "rm"
	}
	return r.applet
}
