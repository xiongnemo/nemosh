package runtime

import (
	"fmt"
	"strings"
)

// The directory stack: `pushd`, `popd`, `dirs`.
//
// **Position zero is not stored.** bash's stack always has the current directory at the
// top, which means anything that keeps its own copy has to be told every time `cd` moves
// -- and the one that forgets is the bug, because `cd /tmp; dirs` then shows the old
// directory at the top and nothing else disagrees. So what is stored is only what lies
// *beneath* the current directory, and position zero is read from the shell. It cannot go
// stale because there is nothing to keep in step.
//
// A subshell gets a copy, so `(pushd /tmp)` leaves the parent where it was. That is bash's
// behaviour and the same rule arrays follow; `$SECONDS` and history are the deliberate
// exceptions, and a directory stack is not one of them.

// directoryStack is what lies beneath the current directory, nearest first.
type directoryStack struct {
	below []string
}

func newDirectoryStack() *directoryStack { return &directoryStack{} }

func (d *directoryStack) clone() *directoryStack {
	return &directoryStack{below: append([]string(nil), d.below...)}
}

// entries is the whole stack as bash prints it, current directory first.
func (r Runtime) directoryEntries() []string {
	return append([]string{r.WorkingDirectory()}, r.dirStack.below...)
}

// dirsFormat is how the stack is being asked for.
type dirsFormat struct {
	numbered bool
	perLine  bool
	// long leaves the home directory spelled out instead of as `~`.
	long bool
}

func (r Runtime) printDirectoryStack(format dirsFormat) int {
	entries := r.directoryEntries()
	if !format.perLine {
		shown := make([]string, 0, len(entries))
		for _, entry := range entries {
			shown = append(shown, r.abbreviateHome(entry, format.long))
		}
		fmt.Fprintln(r.streams.Stdout, strings.Join(shown, " "))
		return 0
	}
	for index, entry := range entries {
		if format.numbered {
			fmt.Fprintf(r.streams.Stdout, "%2d  %s\n", index, r.abbreviateHome(entry, format.long))
			continue
		}
		fmt.Fprintln(r.streams.Stdout, r.abbreviateHome(entry, format.long))
	}
	return 0
}

// abbreviateHome shortens the home directory to `~`, which is what makes a stack of four
// deep paths readable on one line.
func (r Runtime) abbreviateHome(path string, long bool) string {
	home := r.vars["HOME"]
	if long || home == "" || !strings.HasPrefix(path, home) {
		return path
	}
	if len(path) == len(home) {
		return "~"
	}
	if rest := path[len(home):]; rest[0] == '/' || rest[0] == '\\' {
		return "~" + rest
	}
	return path
}

// resolveDirStackOffset turns a raw offset into a position in the printed stack.
//
// `-N` counts from the far end, so `-0` is the *oldest* entry -- which dirStackOffset
// reports as a negative number and every caller has to fold against the length. Doing it
// in one place is not tidiness: `pushd +N` folded it and `dirs -0` did not, so the two
// disagreed about what the same operand meant.
func resolveDirStackOffset(index, length int) (int, bool) {
	if index < 0 {
		index += length
	}
	if index < 0 || index >= length {
		return 0, false
	}
	return index, true
}

// exchangeTopDirectories is a bare pushd, which exchanges the top two: the form people use as a
// there-and-back. With nothing beneath, there is nothing to exchange with, and bash says so
// rather than doing nothing.
func (r Runtime) exchangeTopDirectories() int {
	if len(r.dirStack.below) == 0 {
		fmt.Fprintln(r.streams.Stderr, r.diagnosticPrefix()+"pushd: no other directory")
		return 1
	}
	target := r.dirStack.below[0]
	r.dirStack.below[0] = r.WorkingDirectory()
	if status := r.changeDirectory("pushd", []string{"--", target}); status != 0 {
		// The exchange is undone, because a failed pushd must leave the stack as it was
		// rather than half-swapped.
		r.dirStack.below[0] = target
		return status
	}
	return r.printDirectoryStack(dirsFormat{})
}

// rotateDirectoryStack is `pushd +N`: the stack is turned so that entry N is on top.
//
// A rotation rather than a removal, which is what surprises people who expect it to
// behave like `popd +N`. bash rotates, so this does. With -n the shell stays where it is and
// the entry it would have moved to is dropped, as bash drops it.
func (r Runtime) rotateDirectoryStack(position int, noChange bool) int {
	entries := r.directoryEntries()
	rotated := append(append([]string{}, entries[position:]...), entries[:position]...)
	r.dirStack.below = rotated[1:]
	if noChange {
		return 0
	}
	if status := r.changeDirectory("pushd", []string{"--", rotated[0]}); status != 0 {
		r.dirStack.below = entries[1:]
		return status
	}
	return r.printDirectoryStack(dirsFormat{})
}
