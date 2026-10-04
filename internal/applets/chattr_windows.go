//go:build windows

package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// chattr is `chattr [-R] [-+rhsatn]... FILE...`, as busybox-w32 takes it: it clears (-) or sets
// (+) the attributes Windows lets a program change -- read only, hidden, system, archive,
// temporary and not indexed -- on each FILE, and with -R on everything under a directory, past
// no link. An R among the letters after a dash is -R, as busybox reads it, and the words of
// letters end at the first FILE. The rest of what lsattr shows is not SetFileAttributes' to
// change.
type chattrRequest struct {
	add, remove uint32
	recurse     bool
	stderr      io.Writer
}

func newChattrApplet() Applet {
	return simpleApplet{name: "chattr", runContext: func(ctx context.Context, args []string, _ io.Reader, _, stderr io.Writer) error {
		request, files, err := parseChattrArguments(args)
		if err != nil {
			return err
		}
		request.stderr = stderr
		view := ProcessViewFromContext(ctx)
		for _, file := range files {
			native, err := resolveHostPath(view, file)
			if err != nil {
				request.report(cannotStat(file, err))
				continue
			}
			request.change(ctx, native, file)
		}
		// 0 whatever was said, as busybox's chattr answers; see report.
		return nil
	}}
}

// parseChattrArguments reads the words of letters and finds the FILEs after them, refusing what
// busybox's chattr refuses: a letter it has not got, no FILE, no letter to change at all, and
// one both set and cleared.
func parseChattrArguments(args []string) (chattrRequest, []string, error) {
	var request chattrRequest
	given := false
	for index, arg := range args {
		if arg == "" || arg[0] != '-' && arg[0] != '+' {
			switch {
			case request.add&request.remove != 0:
				return request, nil, errors.New("an attribute cannot be both set and cleared")
			case !given:
				return request, nil, errors.New("nothing to change: -LETTERS clears attributes and +LETTERS sets them")
			}
			return request, args[index:], nil
		}
		// A + with no letters still says which way, as busybox counts it.
		given = given || arg[0] == '+'
		for _, letter := range []byte(arg[1:]) {
			if arg[0] == '-' && letter == 'R' {
				request.recurse = true
				continue
			}
			value, ok := changeableAttribute(letter)
			if !ok {
				return request, nil, invalidOption(letter)
			}
			if given = true; arg[0] == '+' {
				request.add |= value
			} else {
				request.remove |= value
			}
		}
	}
	return request, nil, missingOperand()
}

// changeableAttribute is the attribute a letter chattr takes stands for.
func changeableAttribute(letter byte) (uint32, bool) {
	for _, flag := range fileAttributeFlags[changeableAttributes:] {
		if flag.letter == letter {
			return flag.value, true
		}
	}
	return 0, false
}

// changeableMask is every attribute chattr can change; the others are left out of what it asks
// SetFileAttributes for, as busybox-w32's CHATTR_MASK leaves them.
func changeableMask() uint32 {
	var mask uint32
	for _, flag := range fileAttributeFlags[changeableAttributes:] {
		mask |= flag.value
	}
	return mask
}

func (r *chattrRequest) change(ctx context.Context, native, shown string) {
	entry, err := readFileEntry(native)
	if err != nil {
		r.report(cannotStat(shown, err))
		return
	}
	attributes := (entry.attributes&^r.remove | r.add) & changeableMask()
	name, err := windows.UTF16PtrFromString(native)
	if err == nil {
		err = windows.SetFileAttributes(name, attributes)
	}
	if err != nil {
		r.report(fmt.Errorf("cannot set the attributes of %s: %s", shown, causeText(err)))
	}
	if !r.recurse || !entry.directory {
		return
	}
	names, err := directoryNames(native)
	if err != nil {
		r.report(cannotOpen(shown, err))
		return
	}
	for _, child := range names {
		if ctx.Err() != nil {
			return
		}
		if child != "." && child != ".." {
			r.change(ctx, filepath.Join(native, child), joinOperand(shown, child))
		}
	}
}

// report says what could not be changed and goes on. The status stays 0, as busybox's is
// whatever happened; it was 1, as in e2fsprogs.
func (r *chattrRequest) report(err error) {
	fmt.Fprintf(r.stderr, "chattr: %v\n", err)
}
