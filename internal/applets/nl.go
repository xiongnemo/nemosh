package applets

import (
	"context"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// nl is busybox's (coreutils/nl.c): `nl [-p] [-b STYLE] [-i N] [-s STRING] [-v N] [-w N]
// [FILE]...`, with --body-numbering, --line-increment, --no-renumber, --number-separator,
// --starting-line-number and --number-width for the letters. A line numbered is its number
// right-aligned in -w columns, 6, then -s, a tab, then the line; one that is not has as many
// blanks in their place. Numbers begin at -v, 1, go up by -i, 1, and carry on from one FILE
// to the next. -b a numbers every line, t the ones that are not empty, which is the default,
// and n none, each by its first letter as busybox reads it; pBRE the ones BRE matches, as
// GNU's does, where busybox numbers none. -p is taken and changes nothing: there are no
// logical pages to number again.
//
// It took -b alone, and read a line of blanks as empty, where busybox numbers it.
func newNlApplet() Applet {
	return simpleApplet{name: "nl", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, paths, err := parseAppletOptions(ctx, longOptionWords(args, nlLongOptions), "p", "wsvib")
		if err != nil {
			return err
		}
		numbering, err := newNlNumbering(options)
		if err != nil {
			return err
		}
		return eachTextFile(ctx, paths, stdin, func(reader io.Reader) error {
			return eachLine(reader, func(line, _ string) error {
				return numbering.write(stdout, line)
			})
		})
	}}
}

var nlLongOptions = map[string]string{"body-numbering": "b", "line-increment": "i", "no-renumber": "p",
	"number-separator": "s", "starting-line-number": "v", "number-width": "w"}

// nlNumbering is busybox's number_state.
type nlNumbering struct {
	width           int
	separator       string
	next, increment uint32
	numbered        func(line string) bool
}

func newNlNumbering(options appletOptions) (*nlNumbering, error) {
	numbering := &nlNumbering{width: 6, separator: "\t", next: 1, increment: 1}
	if options.has('s') {
		numbering.separator = options.value('s')
	}
	for _, letter := range []byte("wvi") {
		for _, text := range options.all(letter) {
			value, err := positiveNumber(text)
			if err != nil {
				return nil, err
			}
			switch letter {
			case 'w':
				numbering.width = value
			case 'v':
				numbering.next = uint32(value)
			case 'i':
				numbering.increment = uint32(value)
			}
		}
	}
	style := "t"
	if options.has('b') {
		style = options.value('b')
	}
	switch {
	case strings.HasPrefix(style, "a"):
		numbering.numbered = func(string) bool { return true }
	case strings.HasPrefix(style, "t"):
		numbering.numbered = func(line string) bool { return line != "" }
	case strings.HasPrefix(style, "n"):
		numbering.numbered = func(string) bool { return false }
	case strings.HasPrefix(style, "p"):
		pattern, err := compileSedPattern(style[1:], false, false)
		if err != nil {
			return nil, err
		}
		numbering.numbered = pattern.MatchString
	default:
		// busybox numbers none, which is a style nobody asked for by that name.
		return nil, fmt.Errorf("invalid body numbering style: '%s'", style)
	}
	return numbering, nil
}

// write is busybox's print_numbered_lines for one line. The number is unsigned and wraps, as
// busybox's does.
func (n *nlNumbering) write(stdout io.Writer, line string) error {
	if !n.numbered(line) {
		if err := writeBlanks(stdout, n.width+len(n.separator)); err != nil {
			return err
		}
	} else {
		digits := strconv.FormatUint(uint64(n.next), 10)
		if err := writeBlanks(stdout, n.width-len(digits)); err != nil {
			return err
		}
		if _, err := io.WriteString(stdout, digits+n.separator); err != nil {
			return err
		}
		n.next += n.increment
	}
	_, err := io.WriteString(stdout, line+"\n")
	return err
}

// writeBlanks writes count blanks a block at a time, so a width of a billion costs that much
// output and not that much memory in the shell the applet runs in.
func writeBlanks(stdout io.Writer, count int) error {
	for count > 0 {
		chunk := min(count, len(blankBlock))
		if _, err := io.WriteString(stdout, blankBlock[:chunk]); err != nil {
			return err
		}
		count -= chunk
	}
	return nil
}

var blankBlock = strings.Repeat(" ", 4096)

// positiveNumber is busybox's xatoi_positive: decimal digits and nothing else, no sign and no
// blank before them, from 0 to INT_MAX.
func positiveNumber(text string) (int, error) {
	value, err := strconv.ParseUint(text, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid number '%s'", text)
	}
	if value > math.MaxInt32 {
		return 0, fmt.Errorf("number %s is not in 0..%d range", text, math.MaxInt32)
	}
	return int(value), nil
}
