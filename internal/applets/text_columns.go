package applets

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/xiongnemo/nemosh/internal/textgrid"
)

// expand and unexpand.
//
// Grouped because both are about the shape of a line rather than its bytes.
// join is in join.go, and base32 and shuf are in text_random.go.
// Measured against busybox-w32 v1.38.0 on 2026-08-22.

// newExpandApplet is busybox's expand (coreutils/expand.c): each tab made the spaces to the next
// stop, every -t columns, 8 unless it says, and with -i only the tabs before a line's first
// character that is neither a space nor a tab, which leaves one inside a string literal. The
// ending is kept, so a CRLF file stays CRLF and one with no last newline stays so.
//
// Columns are counted as busybox's unicode_strwidth counts them, in the cells a terminal draws:
// a tab after 一二 is four spaces. busybox-w32 counts bytes, its unicode support being off,
// and this counted runes, which put the tab two cells late.
func newExpandApplet() Applet {
	return simpleApplet{name: "expand", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, paths, err := parseAppletLongOptions(ctx, args, map[string]string{"initial": "i", "tabs": "t"}, "i", "t")
		if err != nil {
			return err
		}
		stop, err := tabStopWidth(options)
		if err != nil {
			return err
		}
		return eachTextFile(ctx, paths, stdin, func(reader io.Reader) error {
			// Each line is written as it is read, so a pipe from something still running
			// answers as it goes.
			out := bufio.NewWriter(stdout)
			return eachLine(reader, func(line, ending string) error {
				return errors.Join(expandTabs(out, line+ending, stop, options.has('i')), out.Flush())
			})
		})
	}}
}

// expandTabs is busybox's expand of a line: the text since the tab before, and the spaces from
// its width to the next stop, which is where the tab before ended.
func expandTabs(out *bufio.Writer, line string, stop int, initialOnly bool) error {
	start := 0
	for index := 0; index < len(line); index++ {
		c := line[index]
		if initialOnly && c != ' ' && c != '\t' {
			break
		}
		if c == '\t' {
			out.WriteString(line[start:index])
			writeSpaces(out, stop-textgrid.Cells(line[start:index])%stop)
			start = index + 1
		}
	}
	_, err := out.WriteString(line[start:])
	return err
}

// writeSpaces writes n blanks as they go, so that a -t in the billions is a lot of output
// rather than a line held whole.
func writeSpaces(out *bufio.Writer, n int) {
	for ; n > 0; n-- {
		out.WriteByte(' ')
	}
}

// newUnexpandApplet is busybox's unexpand: each run of blanks a tab for each stop it reaches and
// spaces for the rest, the leading ones of a line, and with -a all of them. -t sets -a as well,
// as busybox's getopt32 has it, and -f takes it back.
func newUnexpandApplet() Applet {
	return simpleApplet{name: "unexpand", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, paths, err := parseAppletLongOptions(ctx, args, map[string]string{"first-only": "f", "tabs": "t", "all": "a"}, "fa", "t")
		if err != nil {
			return err
		}
		stop, err := tabStopWidth(options)
		if err != nil {
			return err
		}
		all := (options.has('a') || options.has('t')) && !options.has('f')
		return eachTextFile(ctx, paths, stdin, func(reader io.Reader) error {
			out := bufio.NewWriter(stdout)
			return eachLine(reader, func(line, ending string) error {
				return errors.Join(unexpandTabs(out, line+ending, stop, all), out.Flush())
			})
		})
	}}
}

// unexpandTabs is busybox's unexpand of a line, its newline in it. Without all it is done after
// the first run of blanks, or when the line begins with text, the run after that word, which
// busybox changes too where GNU does not. A tab it passes is put back as the stops come, so
// one at the end of a last line with no newline is lost, as busybox loses it.
func unexpandTabs(out *bufio.Writer, line string, stop int, all bool) error {
	at, column := 0, 0
	for at < len(line) {
		spaces := 0
		for at < len(line) && line[at] == ' ' {
			at, spaces = at+1, spaces+1
		}
		column += spaces
		if at < len(line) && line[at] == '\t' {
			column += stop - column%stop
			at++
			continue
		}
		if tabs := column / stop; tabs > 0 {
			for ; tabs > 0; tabs-- {
				out.WriteByte('\t')
			}
			column %= stop
			spaces = column
		}
		writeSpaces(out, spaces)
		if !all && at != 0 {
			_, err := out.WriteString(line[at:])
			return err
		}
		word := strings.IndexAny(line[at:], "\t ")
		if word < 0 {
			word = len(line) - at
		}
		out.WriteString(line[at : at+word])
		column = (column + textgrid.Cells(line[at:at+word])) % stop
		at += word
	}
	return nil
}

// tabStopWidth is -t, as xatou_range(opt_t, 1, UINT_MAX) reads it.
func tabStopWidth(options appletOptions) (int, error) {
	if !options.has('t') {
		return 8, nil
	}
	text := options.value('t')
	value, err := busyboxNumberBase(text, 10, math.MaxUint32, math.MaxUint32, nil)
	if err == nil && value == 0 {
		err = fmt.Errorf("number %s is not in 1..%d range", text, uint64(math.MaxUint32))
	}
	return int(value), err
}
