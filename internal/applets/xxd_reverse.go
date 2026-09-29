package applets

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
)

// xxdReverse is xxd -r, busybox's reverse (util-linux/hexdump_xxd.c:90-220): the bytes of a dump
// in FILE or stdin, each put at the address its line begins with, -s's OFFSET added; with -p
// every pair of hex digits in turn, and no addresses.
func xxdReverse(ctx context.Context, paths []string, stdin io.Reader, stdout io.Writer, plain bool, skip int64) error {
	input := stdin
	if len(paths) == 1 {
		file, err := OpenProcessInput(ctx, ProcessViewFromContext(ctx), paths[0])
		if err != nil {
			return cannotOpen(paths[0], err)
		}
		defer file.Close()
		input = file
	}
	r := &xxdReverser{lines: bufio.NewReader(input), out: bufio.NewWriter(stdout), plain: plain, skip: skip}
	if file := stdoutFile(stdout); file != nil && isRegularFile(file) {
		r.file = file
	}
	err := r.run()
	return errors.Join(err, r.out.Flush())
}

// xxdReverser is reverse's state: where its output is, and what it reads.
type xxdReverser struct {
	lines *bufio.Reader
	out   *bufio.Writer
	// file is stdout when that is a file, which an address seeks in. Anywhere else busybox's
	// fseeko fails and it writes zeros up to the address, and dies at one behind it.
	// busybox-w32's seek on a pipe succeeds without moving, so there a gap is lost and an
	// address behind writes on; this does as busybox does elsewhere.
	file      *os.File
	plain     bool
	skip, cur int64
}

// line is the next line, as xmalloc_fgetline has one: without its newline.
func (r *xxdReverser) line() (string, bool, error) {
	text, err := r.lines.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", false, err
	}
	if text == "" && err != nil {
		return "", false, nil
	}
	return strings.TrimSuffix(text, "\n"), true, nil
}

func (r *xxdReverser) run() error {
	for {
		line, ok, err := r.line()
		if err != nil || !ok {
			return err
		}
		at := 0
		if !r.plain {
			if at, err = r.address(line); err != nil {
				return err
			}
		}
		if err := r.convert(line, at); err != nil {
			return err
		}
	}
}

// convert writes a byte for each two hex digits of line from at. One character that is not a
// digit may stand before a byte, a space between groups, and a second ends the line, as the
// two before the text do; with -p blanks are passed over anywhere. A digit alone is dropped.
// A line that ends between two digits goes on into the next, which with -p finishes the byte,
// and without it begins with an address, so that the first digit is lost.
func (r *xxdReverser) convert(line string, at int) error {
bytes:
	for {
		high, found := byte(0), false
		for bad := 0; !found; bad++ {
			if r.plain {
				at = skipCBlanks(line, at)
			}
			if at >= len(line) {
				return nil
			}
			high, found = hexValue(line[at])
			at++
			if !found && bad == 1 {
				return nil
			}
		}
		for {
			if r.plain {
				at = skipCBlanks(line, at)
			}
			if at < len(line) {
				low, ok := hexValue(line[at])
				at++
				if ok {
					if err := r.out.WriteByte(high<<4 | low); err != nil {
						return err
					}
					r.cur++
					continue bytes
				}
				for at < len(line) && !isHexDigit(line[at]) {
					at++
				}
				if at >= len(line) {
					return nil
				}
				continue bytes
			}
			next, ok, err := r.line()
			if err != nil || !ok {
				return err
			}
			line, at = next, 0
			if !r.plain {
				if at, err = r.address(line); err != nil {
					return err
				}
				continue bytes
			}
		}
	}
}

// address moves the output to where a line's bytes go, the hex number it begins with and
// -s's OFFSET, and is the index after the number and a colon after it.
func (r *xxdReverser) address(line string) (int, error) {
	at := skipCBlanks(line, 0)
	offset, end, ok := xxdAddress(line[at:])
	if !ok || offset < 0 || offset+r.skip < 0 {
		return 0, fmt.Errorf("invalid number '%s'", line[at:])
	}
	if offset += r.skip; offset != r.cur {
		if err := r.seek(offset); err != nil {
			return 0, err
		}
	}
	at += end
	if at < len(line) && line[at] == ':' {
		at++
	}
	return at, nil
}

// xxdAddress is bb_strtoll(text, &end, 16) (libbb/bb_strtonum.c): a -, 0x and hex digits, and
// ok unless the text starts with neither a letter nor a digit, the number overflows, or a
// letter or digit follows it. With no digits it is 0, and ends where it began.
func xxdAddress(text string) (int64, int, bool) {
	first := byte(0)
	if len(text) > 0 {
		first = text[0]
	}
	if first == '-' && len(text) > 1 {
		first = text[1]
	}
	if !isASCIIDigit(first) && (first|0x20 < 'a' || first|0x20 > 'z') {
		return 0, 0, false
	}
	at, negative := 0, text[0] == '-'
	if negative {
		at++
	}
	if at+2 < len(text) && text[at] == '0' && text[at+1]|0x20 == 'x' && isHexDigit(text[at+2]) {
		at += 2
	}
	start, magnitude, overflow := at, uint64(0), false
	for ; at < len(text) && isHexDigit(text[at]); at++ {
		digit, _ := hexValue(text[at])
		overflow = overflow || magnitude > math.MaxInt64>>4
		magnitude = magnitude<<4 | uint64(digit)
	}
	if at == start {
		at = 0
	}
	if overflow || magnitude > math.MaxInt64 ||
		at < len(text) && (isASCIIDigit(text[at]) || text[at]|0x20 >= 'a' && text[at]|0x20 <= 'z') {
		return 0, 0, false
	}
	if negative {
		return -int64(magnitude), at, true
	}
	return int64(magnitude), at, true
}

// seek moves the output to offset: in a file, there; elsewhere by zeros up to it, and not back,
// `cannot seek`.
func (r *xxdReverser) seek(offset int64) error {
	if r.file != nil {
		if err := r.out.Flush(); err != nil {
			return err
		}
		if _, err := r.file.Seek(offset, io.SeekStart); err == nil {
			r.cur = offset
			return nil
		}
	}
	if offset < r.cur {
		return errors.New("cannot seek: Illegal seek")
	}
	var zeros [4096]byte
	for r.cur < offset {
		n := min(offset-r.cur, int64(len(zeros)))
		if _, err := r.out.Write(zeros[:n]); err != nil {
			return err
		}
		r.cur += n
	}
	return nil
}

// hexValue is a hex digit's value.
func hexValue(c byte) (byte, bool) {
	switch {
	case isASCIIDigit(c):
		return c - '0', true
	case c|0x20 >= 'a' && c|0x20 <= 'f':
		return (c | 0x20) - 'a' + 10, true
	}
	return 0, false
}

// skipCBlanks is skip_whitespace: past the characters C's isspace names.
func skipCBlanks(text string, at int) int {
	for at < len(text) && isCSpace(text[at]) {
		at++
	}
	return at
}
