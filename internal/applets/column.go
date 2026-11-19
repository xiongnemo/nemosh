package applets

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/rivo/uniseg"
)

// column, util-linux's (text-utils/column.c), as Debian has it; busybox-w32 has none.
//
// Without -t each line is an entry, and they are filled into columns down and then across, or
// with -x across and then down: each column as wide as the widest entry rounded up to the next
// tab stop, the gap made of tabs, as many columns as the width holds -- -c's, the terminal's,
// COLUMNS', or 80, in that order, as get_terminal_width has it -- and one entry a line when not
// even one fits. With -t the lines are a table's rows: split at runs of spaces and tabs, or at
// each of -s's characters, so `a,,b` is three fields; -l gives the last field the rest of the line;
// the columns are as wide as their widest field, joined by two spaces or -o's separator, the last
// not padded, -R's right-aligned; -N names them in a first line but with -d. Empty lines are left
// out, but with -L. A width is counted in the columns a character takes.

type columnOptions struct {
	table, across, keepEmpty, noHeadings bool
	separators, output                   string
	width, limit                         int
	right                                map[int]bool
	names                                []string
}

const columnLongOptions = "table=t fillrows=x keep-empty-lines=L table-noheadings=d separator=s input-separator=s " +
	"output-separator=o output-width=c table-right=R table-columns=N table-columns-limit=l"

func newColumnApplet() Applet {
	return simpleApplet{name: "column", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, operands, err := parseAppletLongOptions(ctx, args, columnLongOptions, "txLd", "socRNl")
		if err != nil {
			return err
		}
		settings, err := columnSettings(ctx, options, stdout)
		if err != nil {
			return err
		}
		var lines []string
		err = eachTextInput(ctx, operands, stdin, func(reader io.Reader) error {
			return eachLine(reader, func(line, _ string) error {
				if line != "" || settings.keepEmpty {
					lines = append(lines, line)
				}
				return nil
			})
		})
		if err != nil {
			return err
		}
		out := bufio.NewWriter(stdout)
		if settings.table {
			writeColumnTable(out, lines, settings)
		} else {
			writeColumnFill(out, lines, settings)
		}
		return out.Flush()
	}}
}

func columnSettings(ctx context.Context, options appletOptions, stdout io.Writer) (columnOptions, error) {
	settings := columnOptions{
		table: options.has('t'), across: options.has('x'), keepEmpty: options.has('L'), noHeadings: options.has('d'),
		separators: options.value('s'), output: "  ", right: map[int]bool{},
	}
	if options.has('o') {
		settings.output = options.value('o')
	}
	switch value := options.value('c'); {
	case !options.has('c'):
		settings.width = psTerminalWidth(stdout)
		if settings.width <= 0 {
			if columns, ok := ProcessViewFromContext(ctx).LookupEnv("COLUMNS"); ok {
				settings.width, _ = strconv.Atoi(columns)
			}
		}
		if settings.width <= 0 {
			settings.width = 80
		}
	case value == "unlimited":
		settings.width = 0
	default:
		width, err := strconv.Atoi(value)
		if err != nil || width < 0 {
			return settings, fmt.Errorf("invalid columns argument: '%s'", value)
		}
		settings.width = width
	}
	if options.has('l') {
		limit, err := strconv.Atoi(options.value('l'))
		if err != nil || limit < 1 {
			return settings, fmt.Errorf("invalid columns limit argument: '%s'", options.value('l'))
		}
		settings.limit = limit
	}
	if options.has('N') {
		settings.names = strings.Split(options.value('N'), ",")
	}
	for _, column := range strings.Split(options.value('R'), ",") {
		if column == "" {
			continue
		}
		number, err := strconv.Atoi(column)
		if err != nil || number < 1 {
			if index := indexOf(settings.names, column); index >= 0 {
				number = index + 1
			} else {
				return settings, errors.New("undefined column name '" + column + "'")
			}
		}
		settings.right[number-1] = true
	}
	return settings, nil
}

// writeColumnFill is columnate_fillcols, or with -x columnate_fillrows.
func writeColumnFill(out *bufio.Writer, entries []string, settings columnOptions) {
	widest := 0
	for _, entry := range entries {
		widest = max(widest, uniseg.StringWidth(entry))
	}
	widest = (widest + 8) &^ 7
	if settings.width < widest {
		for _, entry := range entries {
			out.WriteString(entry + "\n")
		}
		return
	}
	columns := max(1, settings.width/widest)
	rows := (len(entries) + columns - 1) / columns
	at := func(row, column int) int { return column*rows + row }
	if settings.across {
		rows = (len(entries) + columns - 1) / columns
		at = func(row, column int) int { return row*columns + column }
	}
	for row := range rows {
		written, end := 0, widest
		for column := range columns {
			index := at(row, column)
			if index >= len(entries) {
				break
			}
			if column > 0 {
				for written < end-widest {
					out.WriteByte('\t')
					written = (written + 8) &^ 7
				}
			}
			out.WriteString(entries[index])
			written += uniseg.StringWidth(entries[index])
			end += widest
		}
		out.WriteByte('\n')
	}
}

func indexOf(names []string, name string) int {
	for index, candidate := range names {
		if candidate == name {
			return index
		}
	}
	return -1
}
