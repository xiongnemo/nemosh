package applets

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// newFoldApplet wraps long lines, as busybox's fold does: a line is cut before the character
// that would take it past -w's width in columns, default 80. A tab reaches the next multiple
// of eight, a backspace goes back one and a carriage return goes back to the start; -b counts
// every byte as one column instead. -s cuts after the last blank before that, a space or a
// tab, where there is one. A line's first character is never cut off, however wide.
//
// It counted runes, a tab one of them, so `a<TAB>bc` was three columns where busybox's is ten,
// and -b was taken and ignored. A character is still a rune here, so a line is not cut through
// one; under -b a byte is, as busybox cuts it.
func newFoldApplet() Applet {
	return simpleApplet{name: "fold", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, paths, err := parseAppletOptions(ctx, args, "bs", "w")
		if err != nil {
			return err
		}
		width := 80
		if options.has('w') {
			parsed, err := strconv.Atoi(options.value('w'))
			if err != nil || parsed <= 0 {
				return fmt.Errorf("illegal width value '%s'", options.value('w'))
			}
			width = parsed
		}
		folding := folder{width: width, atBlanks: options.has('s'), bytes: options.has('b')}
		return eachTextFile(ctx, paths, stdin, func(reader io.Reader) error {
			// The breaks fold *inserts* are newlines; the ending the input had goes
			// on the last piece only. Measured against busybox, which is the only
			// way to know: folding a CRLF line at width three answers three pieces,
			// the first two ended by a bare newline and the last keeping the CRLF --
			// so the file keeps its endings at the real ends of lines and gets plain
			// newlines only where a line was cut.
			return eachLine(reader, func(line, ending string) error {
				pieces := folding.fold(line)
				for index, piece := range pieces {
					terminator := "\n"
					if index == len(pieces)-1 {
						terminator = ending
					}
					if _, err := io.WriteString(stdout, piece+terminator); err != nil {
						return err
					}
				}
				return nil
			})
		})
	}}
}

// folder is how fold cuts: the width, -s and -b.
type folder struct {
	width           int
	atBlanks, bytes bool
}

// fold cuts one line into pieces, as busybox's fold_main does. An empty line yields one empty
// piece rather than none, because dropping it would lose a line.
func (f folder) fold(line string) []string {
	var units []string
	if f.bytes {
		for index := 0; index < len(line); index++ {
			units = append(units, line[index:index+1])
		}
	} else {
		for _, character := range line {
			units = append(units, string(character))
		}
	}
	var pieces, current []string
	column := 0
	for index := 0; index < len(units); {
		next := f.column(column, units[index])
		if next <= f.width || len(current) == 0 {
			current, column = append(current, units[index]), next
			index++
			continue
		}
		// The character starts the next piece, or under -s what follows the last blank does.
		cut := len(current)
		if f.atBlanks {
			if blank := lastBlank(current); blank > 0 {
				cut = blank
			}
		}
		pieces = append(pieces, strings.Join(current[:cut], ""))
		current = append([]string(nil), current[cut:]...)
		column = 0
		for _, unit := range current {
			column = f.column(column, unit)
		}
	}
	return append(pieces, strings.Join(current, ""))
}

// column is busybox's adjust_column: where the column is after a character.
func (f folder) column(column int, unit string) int {
	switch {
	case f.bytes:
		return column + 1
	case unit == "\t":
		return column + 8 - column%8
	case unit == "\b":
		return max(column-1, 0)
	case unit == "\r":
		return 0
	}
	return column + 1
}

// lastBlank is how much of a piece runs to its last space or tab, the blank included, or 0
// where it has none.
func lastBlank(units []string) int {
	for index := len(units) - 1; index >= 0; index-- {
		if units[index] == " " || units[index] == "\t" {
			return index + 1
		}
	}
	return 0
}
