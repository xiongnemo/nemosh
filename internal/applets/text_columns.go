package applets

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/xiongnemo/nemosh/internal/textgrid"
)

// expand, unexpand and join.
//
// Grouped because all three are about the shape of a line rather than its bytes.
// base32 and shuf are in text_random.go.
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
		words := longOptionWords(args, map[string]string{"initial": "i", "tabs": "t"})
		options, paths, err := parseAppletOptions(ctx, words, "i", "t")
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
		words := longOptionWords(args, map[string]string{"first-only": "f", "tabs": "t", "all": "a"})
		options, paths, err := parseAppletOptions(ctx, words, "fa", "t")
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

// newJoinApplet joins two sorted files on a common field.
//
// The default field is the first and the separator is any run of blanks, which is
// POSIX's default and busybox's.
func newJoinApplet() Applet {
	return simpleApplet{name: "join", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, operands, err := parseAppletOptions(ctx, args, "", "j1 2t")
		if err != nil {
			return err
		}
		if len(operands) != 2 {
			return fmt.Errorf("join: two file operands are required")
		}
		leftField, rightField, err := joinFields(options)
		if err != nil {
			return err
		}
		left, err := readJoinLines(ctx, operands[0], stdin)
		if err != nil {
			return err
		}
		right, err := readJoinLines(ctx, operands[1], stdin)
		if err != nil {
			return err
		}
		return writeJoined(stdout, left, right, leftField, rightField)
	}}
}

// joinFields reads which field to join on, **per file**.
//
// -1 and -2 are separate on purpose: `join -1 2 -2 1` joins the second field of
// the first file to the first field of the second, and both references agree.
// Collapsing them into one number, which is what this did first, silently
// answered nothing for every asymmetric join.
//
// -j sets both, which is GNU's shorthand. busybox does not have -j; offering it
// is the smaller divergence, since refusing a standard option is worse than
// having one the reference lacks.
func joinFields(options appletOptions) (int, int, error) {
	left, right := 1, 1
	read := func(letter byte) (int, error) {
		parsed, err := strconv.Atoi(options.value(letter))
		if err != nil || parsed <= 0 {
			return 0, fmt.Errorf("invalid field number '%s'", options.value(letter))
		}
		return parsed, nil
	}
	if options.has('j') {
		both, err := read('j')
		if err != nil {
			return 0, 0, err
		}
		left, right = both, both
	}
	if options.has('1') {
		parsed, err := read('1')
		if err != nil {
			return 0, 0, err
		}
		left = parsed
	}
	if options.has('2') {
		parsed, err := read('2')
		if err != nil {
			return 0, 0, err
		}
		right = parsed
	}
	return left, right, nil
}

func readJoinLines(ctx context.Context, path string, stdin io.Reader) ([][]string, error) {
	var rows [][]string
	err := eachTextInput(ctx, []string{path}, stdin, func(reader io.Reader) error {
		return eachLine(reader, func(line, _ string) error {
			rows = append(rows, strings.Fields(line))
			return nil
		})
	})
	return rows, err
}

// writeJoined emits `key rest-of-left rest-of-right` for every pair whose keys
// match, which is what makes `join` a relational join rather than a paste.
func writeJoined(stdout io.Writer, left, right [][]string, leftField, rightField int) error {
	for _, leftRow := range left {
		key, ok := joinKey(leftRow, leftField)
		if !ok {
			continue
		}
		for _, rightRow := range right {
			rightKey, ok := joinKey(rightRow, rightField)
			if !ok || rightKey != key {
				continue
			}
			pieces := append([]string{key}, joinRest(leftRow, leftField)...)
			pieces = append(pieces, joinRest(rightRow, rightField)...)
			if _, err := fmt.Fprintln(stdout, strings.Join(pieces, " ")); err != nil {
				return err
			}
		}
	}
	return nil
}

func joinKey(row []string, field int) (string, bool) {
	if len(row) < field {
		return "", false
	}
	return row[field-1], true
}

func joinRest(row []string, field int) []string {
	rest := make([]string, 0, len(row))
	for index, value := range row {
		if index != field-1 {
			rest = append(rest, value)
		}
	}
	return rest
}
