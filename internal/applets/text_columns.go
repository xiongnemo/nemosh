package applets

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// expand, unexpand and join.
//
// Grouped because all three are about the shape of a line rather than its bytes.
// base32 and shuf are in text_random.go.
// Measured against busybox-w32 v1.38.0 on 2026-08-22.

// newExpandApplet turns tabs into spaces. -t sets the stop width, default 8;
// -i stops converting after the first non-blank, which is what makes it safe on
// source code where a tab inside a string literal must survive.
func newExpandApplet() Applet {
	return simpleApplet{name: "expand", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, paths, err := parseAppletOptions(args, "i", "t")
		if err != nil {
			return err
		}
		stop, err := tabStopWidth(options)
		if err != nil {
			return err
		}
		return eachTextInput(ctx, paths, stdin, func(reader io.Reader) error {
			// The ending is written back rather than replaced by a newline:
			// expand changes tabs to spaces and nothing else, so a CRLF file stays
			// CRLF and a file with no final newline stays that way. Measured
			// against busybox and GNU, which both preserve both.
			return eachLine(reader, func(line, ending string) error {
				_, err := io.WriteString(stdout, expandTabs(line, stop, options.has('i'))+ending)
				return err
			})
		})
	}}
}

// expandTabs replaces each tab with spaces up to the next stop.
//
// The column is counted in runes, so a tab after CJK text lands where it looks
// like it should. Counting bytes would put it three columns early for every
// three-byte character.
func expandTabs(line string, stop int, initialOnly bool) string {
	var out strings.Builder
	column := 0
	blanksOnly := true
	for _, character := range line {
		if character != '\t' {
			if character != ' ' {
				blanksOnly = false
			}
			out.WriteRune(character)
			column++
			continue
		}
		if initialOnly && !blanksOnly {
			out.WriteRune('\t')
			column++
			continue
		}
		width := stop - column%stop
		out.WriteString(strings.Repeat(" ", width))
		column += width
	}
	return out.String()
}

// newUnexpandApplet turns spaces back into tabs. Only leading blanks by default;
// -a converts throughout, which is the form that damages aligned comments and is
// therefore not the default in either reference.
func newUnexpandApplet() Applet {
	return simpleApplet{name: "unexpand", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, paths, err := parseAppletOptions(args, "a", "t")
		if err != nil {
			return err
		}
		stop, err := tabStopWidth(options)
		if err != nil {
			return err
		}
		return eachTextInput(ctx, paths, stdin, func(reader io.Reader) error {
			// The ending preserved, for the same reason as expand above.
			return eachLine(reader, func(line, ending string) error {
				_, err := io.WriteString(stdout, unexpandTabs(line, stop, options.has('a'))+ending)
				return err
			})
		})
	}}
}

func unexpandTabs(line string, stop int, everywhere bool) string {
	runes := []rune(line)
	var out strings.Builder
	column, run := 0, 0
	flush := func() {
		// A run of spaces becomes tabs only where it actually reaches a stop;
		// the remainder stays as spaces, or the text after it would shift.
		for run > 0 {
			boundary := stop - (column-run)%stop
			if run >= boundary && boundary > 1 {
				out.WriteRune('\t')
				run -= boundary
				continue
			}
			out.WriteString(strings.Repeat(" ", run))
			run = 0
		}
	}
	for index, character := range runes {
		if character == ' ' && (everywhere || onlyBlanksBefore(runes[:index])) {
			run++
			column++
			continue
		}
		flush()
		out.WriteRune(character)
		column++
	}
	flush()
	return out.String()
}

func onlyBlanksBefore(runes []rune) bool {
	for _, character := range runes {
		if character != ' ' && character != '\t' {
			return false
		}
	}
	return true
}

func tabStopWidth(options appletOptions) (int, error) {
	if !options.has('t') {
		return 8, nil
	}
	parsed, err := strconv.Atoi(options.value('t'))
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("invalid tab size '%s'", options.value('t'))
	}
	return parsed, nil
}

// newJoinApplet joins two sorted files on a common field.
//
// The default field is the first and the separator is any run of blanks, which is
// POSIX's default and busybox's.
func newJoinApplet() Applet {
	return simpleApplet{name: "join", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, operands, err := parseAppletOptions(args, "", "j1 2t")
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
