package runtime

import (
	"fmt"
	"strconv"
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

// pushd puts the current directory on the stack and changes to another.
func (r Runtime) pushd(args []string) int {
	if index, ok, err := dirStackOffset(args); err != nil {
		fmt.Fprintf(r.streams.Stderr, "pushd: %v\n", err)
		return 1
	} else if ok {
		return r.rotateDirectoryStack(index)
	}
	switch len(args) {
	case 0:
		// No operand exchanges the top two, which is the form people use as a
		// there-and-back. With nothing beneath, there is nothing to exchange with, and
		// bash says so rather than doing nothing.
		if len(r.dirStack.below) == 0 {
			fmt.Fprintln(r.streams.Stderr, "pushd: no other directory")
			return 1
		}
		target := r.dirStack.below[0]
		r.dirStack.below[0] = r.WorkingDirectory()
		if status := r.changeDirectory("pushd", []string{target}); status != 0 {
			// The exchange is undone, because a failed pushd must leave the stack as
			// it was rather than half-swapped.
			r.dirStack.below[0] = target
			return status
		}
	case 1:
		previous := r.WorkingDirectory()
		if status := r.changeDirectory("pushd", args); status != 0 {
			return status
		}
		r.dirStack.below = append([]string{previous}, r.dirStack.below...)
	default:
		fmt.Fprintln(r.streams.Stderr, "pushd: too many arguments")
		return 1
	}
	return r.printDirectoryStack(dirsFormat{})
}

// popd removes an entry, changing directory when it was the current one.
func (r Runtime) popd(args []string) int {
	index, hasIndex, err := dirStackOffset(args)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "popd: %v\n", err)
		return 1
	}
	if !hasIndex && len(args) > 0 {
		fmt.Fprintln(r.streams.Stderr, "popd: too many arguments")
		return 1
	}
	if len(r.dirStack.below) == 0 {
		fmt.Fprintln(r.streams.Stderr, "popd: directory stack empty")
		return 1
	}
	if !hasIndex {
		index = 0
	}
	entries := r.directoryEntries()
	index, inRange := resolveDirStackOffset(index, len(entries))
	if !inRange {
		fmt.Fprintf(r.streams.Stderr, "popd: %s: directory stack index out of range\n", args[0])
		return 1
	}
	if index == 0 {
		// Removing the current directory means moving to the one under it.
		target := r.dirStack.below[0]
		if status := r.changeDirectory("popd", []string{target}); status != 0 {
			return status
		}
		r.dirStack.below = r.dirStack.below[1:]
		return r.printDirectoryStack(dirsFormat{})
	}
	// Anything else is removed without moving: the shell stays where it is.
	r.dirStack.below = append(r.dirStack.below[:index-1], r.dirStack.below[index:]...)
	return r.printDirectoryStack(dirsFormat{})
}

// dirsFormat is how the stack is being asked for.
type dirsFormat struct {
	numbered bool
	perLine  bool
	// long leaves the home directory spelled out instead of as `~`.
	long bool
}

// dirs prints the stack.
func (r Runtime) dirs(args []string) int {
	format := dirsFormat{}
	var offsets []string
	for _, arg := range args {
		if strings.HasPrefix(arg, "+") || strings.HasPrefix(arg, "-") && len(arg) > 1 && arg[1] >= '0' && arg[1] <= '9' {
			offsets = append(offsets, arg)
			continue
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			fmt.Fprintf(r.streams.Stderr, "dirs: %s: invalid argument\n", arg)
			return 1
		}
		for _, letter := range arg[1:] {
			switch letter {
			case 'c':
				r.dirStack.below = nil
				return 0
			case 'v':
				format.numbered, format.perLine = true, true
			case 'p':
				format.perLine = true
			case 'l':
				format.long = true
			default:
				fmt.Fprintf(r.streams.Stderr, "dirs: -%c: invalid option\n", letter)
				return 1
			}
		}
	}
	if len(offsets) > 0 {
		return r.printOneDirectory(offsets, format)
	}
	return r.printDirectoryStack(format)
}

func (r Runtime) printOneDirectory(offsets []string, format dirsFormat) int {
	entries := r.directoryEntries()
	for _, offset := range offsets {
		raw, _, err := dirStackOffset([]string{offset})
		index, inRange := resolveDirStackOffset(raw, len(entries))
		if err != nil || !inRange {
			fmt.Fprintf(r.streams.Stderr, "dirs: %s: directory stack index out of range\n", offset)
			return 1
		}
		fmt.Fprintln(r.streams.Stdout, r.abbreviateHome(entries[index], format.long))
	}
	return 0
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

// dirStackOffset reads a `+N` or `-N` operand. `-N` counts from the far end, so `-0` is
// the oldest entry and `+0` the current directory; the negative form comes back as a
// negative number for resolveDirStackOffset to fold.
func dirStackOffset(args []string) (index int, ok bool, err error) {
	if len(args) != 1 {
		return 0, false, nil
	}
	text := args[0]
	if len(text) < 2 || (text[0] != '+' && text[0] != '-') {
		return 0, false, nil
	}
	number, convErr := strconv.Atoi(text[1:])
	if convErr != nil || number < 0 {
		return 0, false, nil
	}
	if text[0] == '+' {
		return number, true, nil
	}
	return -number - 1, true, nil
}

// rotateDirectoryStack is `pushd +N`: the stack is turned so that entry N is on top.
//
// A rotation rather than a removal, which is what surprises people who expect it to
// behave like `popd +N`. bash rotates, so this does.
func (r Runtime) rotateDirectoryStack(index int) int {
	entries := r.directoryEntries()
	index, inRange := resolveDirStackOffset(index, len(entries))
	if !inRange {
		fmt.Fprintln(r.streams.Stderr, "pushd: directory stack index out of range")
		return 1
	}
	rotated := append(append([]string{}, entries[index:]...), entries[:index]...)
	target := rotated[0]
	r.dirStack.below = append([]string{}, rotated[1:]...)
	if status := r.changeDirectory("pushd", []string{target}); status != 0 {
		r.dirStack.below = entries[1:]
		return status
	}
	return r.printDirectoryStack(dirsFormat{})
}
