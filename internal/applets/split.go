package applets

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
)

// split is busybox's (coreutils/split.c): `split [-l N | -b N[b|k|m|g]] [-a N] [INPUT [PREFIX]]`.
// INPUT, - or none for stdin, goes to files named PREFIX, x, and -a letters, 2, counting aa ab
// ... zz: -l lines to a file, 1000, or -b bytes, the number times 512 1024 1048576 or
// 1073741824 for a b k m or g after it. The bytes go as they come, so a CRLF line stays one
// and the last line keeps what it ended with, or did not, and a file is begun only when there
// is something to put in it. Of -l and -b, -b says what is counted and the last given its
// number, as busybox keeps one count for both.
//
// It took -l alone, joined each file's lines with a newline, so a CRLF file came out LF and an
// unterminated last line terminated, and read the whole of INPUT before writing a file.
func newSplitApplet() Applet {
	return simpleApplet{name: "split", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, operands, err := parseAppletOptions(ctx, args, "", "lba")
		if err != nil {
			return err
		}
		spec := splitSpec{prefix: "x", suffixLen: 2, count: 1000, byBytes: options.has('b')}
		if letter := options.last("lb"); letter != 0 {
			if spec.count, err = splitCount(options.value(letter), spec.byBytes); err != nil {
				return err
			}
		}
		if options.has('a') {
			if spec.suffixLen, err = positiveNumber(options.value('a')); err != nil {
				return err
			}
		}
		if len(operands) > 2 {
			return fmt.Errorf("extra operand '%s'", operands[2])
		}
		source := "-"
		if len(operands) > 0 {
			source = operands[0]
		}
		if len(operands) > 1 {
			spec.prefix = operands[1]
		}
		// busybox's NAME_MAX, which it compares with the whole of PREFIX.
		if len(spec.prefix)+spec.suffixLen > 255 {
			return errors.New("suffix too long")
		}
		view := ProcessViewFromContext(ctx)
		input, err := OpenProcessOperand(ctx, view, source, stdin)
		if err != nil {
			return cannotOpen(source, err)
		}
		return errors.Join(splitInput(ctx, view, input, spec), input.Close())
	}}
}

type splitSpec struct {
	prefix    string
	suffixLen int
	count     uint64
	byBytes   bool
}

// splitCount is busybox's reading of -l's number, digits up to 2^63-1, and of -b's, digits and
// then one of b k m g. A count of 0 is refused, as GNU refuses it: busybox's split makes every
// name it has, empty, for -b 0, and puts everything in the first for -l 0.
func splitCount(text string, byBytes bool) (uint64, error) {
	digits, multiplier := text, uint64(1)
	if byBytes && text != "" {
		for _, suffix := range []struct {
			letter byte
			times  uint64
		}{{'b', 512}, {'k', 1024}, {'m', 1024 * 1024}, {'g', 1024 * 1024 * 1024}} {
			if text[len(text)-1] == suffix.letter {
				digits, multiplier = text[:len(text)-1], suffix.times
			}
		}
	}
	value, err := strconv.ParseUint(digits, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number '%s'", text)
	}
	limit := uint64(math.MaxInt64)
	if byBytes {
		limit = math.MaxUint64
	}
	if value > limit/multiplier {
		return 0, fmt.Errorf("number %s is not in 0..%d range", text, limit)
	}
	if value == 0 {
		what := "lines"
		if byBytes {
			what = "bytes"
		}
		return 0, fmt.Errorf("invalid number of %s: '%s'", what, text)
	}
	return value * multiplier, nil
}

// splitInput is busybox's copy loop: a new file whenever the one before has had its count, and
// only once there is something to put in it. The context is asked before each file, which is
// where a long split is stopped: -l 1 over a large INPUT writes a file a line.
func splitInput(ctx context.Context, view ProcessView, input io.Reader, spec splitSpec) error {
	name := []byte(spec.prefix + strings.Repeat("a", spec.suffixLen))
	named := true
	var piece *os.File
	remaining := uint64(0)
	buffer := make([]byte, 16*1024)
	for {
		if err := ctx.Err(); err != nil {
			return closeSplitPiece(piece, err)
		}
		n, readErr := input.Read(buffer)
		for chunk := buffer[:n]; len(chunk) > 0; {
			if remaining == 0 {
				next, err := openSplitPiece(ctx, view, piece, name, named)
				if piece = next; err != nil {
					return closeSplitPiece(piece, err)
				}
				named = nextSplitName(name, spec.suffixLen)
				remaining = spec.count
			}
			write := len(chunk)
			if spec.byBytes {
				write = int(min(uint64(write), remaining))
				remaining -= uint64(write)
			} else if end := bytes.IndexByte(chunk, '\n'); end >= 0 {
				write = end + 1
				remaining--
			}
			if _, err := piece.Write(chunk[:write]); err != nil {
				return closeSplitPiece(piece, err)
			}
			chunk = chunk[write:]
		}
		if errors.Is(readErr, io.EOF) {
			return closeSplitPiece(piece, nil)
		}
		if readErr != nil {
			return closeSplitPiece(piece, readErr)
		}
	}
}

// openSplitPiece closes the file before, if there is one, and begins the next, named name.
func openSplitPiece(ctx context.Context, view ProcessView, before *os.File, name []byte, named bool) (*os.File, error) {
	if !named {
		return before, errors.New("suffixes exhausted")
	}
	if err := ctx.Err(); err != nil {
		return before, err
	}
	if err := closeSplitPiece(before, nil); err != nil {
		return nil, err
	}
	native, err := resolveHostPath(view, string(name))
	if err != nil {
		return nil, err
	}
	file, err := os.Create(native)
	if err != nil {
		return nil, cannotOpen(string(name), err)
	}
	return file, nil
}

func closeSplitPiece(piece *os.File, err error) error {
	if piece == nil {
		return err
	}
	return errors.Join(err, piece.Close())
}

// nextSplitName is busybox's next_file: the suffix's last letter counted up, a z carrying into
// the letter before it, and false once every letter is z. With -a 0 it counts up PREFIX's last
// character, as busybox's does.
func nextSplitName(name []byte, suffixLen int) bool {
	for at := 1; ; at++ {
		if name[len(name)-at] < 'z' {
			name[len(name)-at]++
			return true
		}
		if at+1 > suffixLen {
			return false
		}
		name[len(name)-at] = 'a'
	}
}
