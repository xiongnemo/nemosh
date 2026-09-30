package applets

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// shred is busybox's (coreutils/shred.c): `shred [-fuz] [-n N] [-s SIZE] FILE...` writes random
// bytes over each FILE N times, 3 unless -n says otherwise, flushing after each pass, then zeros
// once more with -z, and removes it with -u. -s SIZE writes that many bytes rather than the
// file's size, octal or hex as C writes them, and past the end too; -f makes a FILE that cannot
// be written writable first. -v and -x are taken and do nothing, as in busybox. An empty FILE is
// not written, and is still removed with -u.
//
// A FILE that cannot be opened ends shred there, as busybox's xopen ends it, with the ones
// before it done and the ones after it untouched.
func newShredApplet() Applet {
	return simpleApplet{name: "shred", runContext: func(ctx context.Context, args []string, _ io.Reader, _, _ io.Writer) error {
		options, operands, err := parseAppletOptions(ctx, args, "fuzvx", "ns")
		if err != nil {
			return err
		}
		if len(operands) == 0 {
			return missingOperand()
		}
		passes := 3
		if options.has('n') {
			if passes, err = strconv.Atoi(options.value('n')); err != nil || passes < 0 {
				return fmt.Errorf("invalid number '%s'", options.value('n'))
			}
		}
		size := int64(-1)
		if options.has('s') {
			if size, err = shredSize(options.value('s')); err != nil {
				return err
			}
		}
		view := ProcessViewFromContext(ctx)
		for _, operand := range operands {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := shredOne(view, operand, options, passes, size); err != nil {
				return err
			}
		}
		return nil
	}}
}

// shredSize is -s's SIZE, read as C's strtoull reads a number in base 0: 0x for hex, a leading 0
// for octal, and nothing after the digits.
func shredSize(text string) (int64, error) {
	if strings.ContainsAny(text, "_oObB") || strings.HasPrefix(text, "-") {
		return 0, fmt.Errorf("invalid size '%s'", text)
	}
	size, err := strconv.ParseInt(text, 0, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size '%s'", text)
	}
	return size, nil
}

// shredOne overwrites one FILE and, with -u, removes it.
func shredOne(view ProcessView, operand string, options appletOptions, passes int, size int64) error {
	native, err := resolveHostPath(view, operand)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(native, os.O_WRONLY, 0)
	if err != nil && options.has('f') {
		// -f: a FILE that cannot be written is made writable, as busybox chmods it to 0666.
		if chmodErr := applyPermissions(native, 0o666, false); chmodErr == nil {
			file, err = os.OpenFile(native, os.O_WRONLY, 0)
		}
	}
	if err != nil {
		return cannotOpen(operand, err)
	}
	if info, statErr := file.Stat(); statErr == nil && info.Size() > 0 {
		length := info.Size()
		if size >= 0 {
			length = size
		}
		for range passes {
			err = errors.Join(err, overwrite(file, rand.Reader, length))
		}
		if options.has('z') {
			err = errors.Join(err, overwrite(file, zeroReader{}, length))
		}
	}
	if options.has('u') {
		err = errors.Join(err, file.Truncate(0))
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return operandFailure(operand, err)
	}
	if options.has('u') {
		if err := os.Remove(native); err != nil {
			return cannotRemove(operand, err)
		}
	}
	return nil
}

// overwrite writes length bytes of source over file from its start, and flushes them.
func overwrite(file *os.File, source io.Reader, length int64) error {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if _, err := io.CopyN(file, source, length); err != nil {
		return err
	}
	if err := file.Sync(); err != nil && !isFlushRefusal(err) {
		return err
	}
	_, err := file.Seek(0, io.SeekStart)
	return err
}

// zeroReader is /dev/zero: an endless run of zero bytes.
type zeroReader struct{}

func (zeroReader) Read(buffer []byte) (int, error) {
	clear(buffer)
	return len(buffer), nil
}
