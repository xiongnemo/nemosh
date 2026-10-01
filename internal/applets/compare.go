package applets

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// comm and paste, which take two inputs at once; cmp is in cmp.go.
//
// Measured against GNU coreutils, which is what these names mean to anyone who
// types them.

// comm reads two sorted files and prints three columns: lines only in the first,
// lines only in the second, lines in both.
//
// Measured, with s1 = a,b,c and s2 = b,c,d:
//
//	a
//	\t\tb
//	\t\tc
//	\td
//
// The indentation is how the column is identified, so suppressing a column with
// -1, -2 or -3 also removes one level from the ones after it.
func newCommApplet() Applet {
	return simpleApplet{name: "comm", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, paths, err := parseAppletOptions(ctx, args, "123", "")
		if err != nil {
			return err
		}
		if len(paths) != 2 {
			return fmt.Errorf("expected two operands, got %d", len(paths))
		}
		view := ProcessViewFromContext(ctx)
		left, err := readOperandLines(ctx, view, paths[0], stdin)
		if err != nil {
			return err
		}
		right, err := readOperandLines(ctx, view, paths[1], stdin)
		if err != nil {
			return err
		}
		show := [3]bool{!options.has('1'), !options.has('2'), !options.has('3')}
		return writeCommColumns(stdout, left, right, show)
	}}
}

func writeCommColumns(stdout io.Writer, left, right []string, show [3]bool) error {
	// The tabs before a column count how many *shown* columns precede it, so
	// suppressing one shifts everything after it left. GNU does the same.
	indent := func(column int) string {
		width := 0
		for earlier := 0; earlier < column; earlier++ {
			if show[earlier] {
				width++
			}
		}
		return strings.Repeat("\t", width)
	}
	emit := func(column int, line string) error {
		if !show[column] {
			return nil
		}
		_, err := io.WriteString(stdout, indent(column)+line+"\n")
		return err
	}
	i, j := 0, 0
	for i < len(left) && j < len(right) {
		switch {
		case left[i] < right[j]:
			if err := emit(0, left[i]); err != nil {
				return err
			}
			i++
		case left[i] > right[j]:
			if err := emit(1, right[j]); err != nil {
				return err
			}
			j++
		default:
			if err := emit(2, left[i]); err != nil {
				return err
			}
			i++
			j++
		}
	}
	for ; i < len(left); i++ {
		if err := emit(0, left[i]); err != nil {
			return err
		}
	}
	for ; j < len(right); j++ {
		if err := emit(1, right[j]); err != nil {
			return err
		}
	}
	return nil
}

// paste joins lines from several files side by side.
//
//	$ paste p1 p2      1\ta
//	$ paste -d, p1 p2  1,a
//	$ paste -s p1      1\t2
//
// `-s` is the different one: instead of reading the files in parallel it puts
// each file's lines on one line of its own.
func newPasteApplet() Applet {
	return simpleApplet{name: "paste", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, paths, err := parseAppletOptions(ctx, args, "s", "d")
		if err != nil {
			return err
		}
		delimiters := []rune{'\t'}
		if options.has('d') {
			if options.value('d') == "" {
				return fmt.Errorf("delimiter list cannot be empty")
			}
			delimiters = pasteDelimiters(options.value('d'))
		}
		view := ProcessViewFromContext(ctx)
		columns := make([][]string, 0, max(len(paths), 1))
		if len(paths) == 0 {
			paths = []string{"-"}
		}
		// Every `-` is the one standard input, read a line to a column in turn, as busybox and
		// POSIX read it: `seq 5 | paste - -` is 1 2, 3 4 and 5. The first `-` took the whole
		// input, so each line came out alone. -s reads each operand to its end, and so a second
		// `-` finds nothing left there too.
		dashes := 0
		for _, path := range paths {
			if path == "-" {
				dashes++
			}
		}
		var shared []string
		dash := 0
		for _, path := range paths {
			if path == "-" && dashes > 1 && !options.has('s') {
				if dash == 0 {
					if shared, err = readOperandLines(ctx, view, path, stdin); err != nil {
						return err
					}
				}
				columns = append(columns, everyNthLine(shared, dash, dashes))
				dash++
				continue
			}
			lines, err := readOperandLines(ctx, view, path, stdin)
			if err != nil {
				return err
			}
			columns = append(columns, lines)
		}
		if options.has('s') {
			for _, lines := range columns {
				// An operand with no lines writes none, as busybox's paste_files_separate
				// writes none, where this wrote an empty one.
				if len(lines) == 0 {
					continue
				}
				if _, err := io.WriteString(stdout, joinWithDelimiters(lines, delimiters)+"\n"); err != nil {
					return err
				}
			}
			return nil
		}
		longest := 0
		for _, lines := range columns {
			longest = max(longest, len(lines))
		}
		for row := range longest {
			fields := make([]string, len(columns))
			for index, lines := range columns {
				if row < len(lines) {
					fields[index] = lines[row]
				}
			}
			if _, err := io.WriteString(stdout, joinWithDelimiters(fields, delimiters)+"\n"); err != nil {
				return err
			}
		}
		return nil
	}}
}

// everyNthLine is the lines one of several `-` columns gets: its own first, then every step-th.
func everyNthLine(lines []string, start, step int) []string {
	var picked []string
	for index := start; index < len(lines); index += step {
		picked = append(picked, lines[index])
	}
	return picked
}

// pasteDelimiters is -d's list, its escapes read as busybox's
// strcpy_and_process_escape_sequences reads them: \t \n \\ and the rest of C's, and \0 a
// delimiter that is nothing. It was taken as written, so `-d '\t,'` put a backslash and a t
// between the columns.
func pasteDelimiters(list string) []rune {
	var delimiters []rune
	for at := 0; at < len(list); {
		if list[at] == '\\' {
			character, used := dumpEscape(list[at+1:])
			delimiters = append(delimiters, rune(character))
			at += 1 + used
			continue
		}
		character, size := utf8.DecodeRuneInString(list[at:])
		delimiters = append(delimiters, character)
		at += size
	}
	return delimiters
}

// joinWithDelimiters cycles through the delimiter list, which is what -d takes:
// `-d,;` alternates comma and semicolon between columns.
func joinWithDelimiters(fields []string, delimiters []rune) string {
	var joined strings.Builder
	for index, field := range fields {
		if index > 0 {
			if delimiter := delimiters[(index-1)%len(delimiters)]; delimiter != 0 {
				joined.WriteRune(delimiter)
			}
		}
		joined.WriteString(field)
	}
	return joined.String()
}

// readOperand reads a named file, or stdin for `-`, which is the convention every
// one of these follows.
func readOperand(ctx context.Context, view ProcessView, path string, stdin io.Reader) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(stdin)
	}
	file, err := OpenProcessInput(ctx, view, path)
	if err != nil {
		// cmp, comm, join and paste open theirs as fopen_or_warn does, which names one it
		// cannot `FILE: No such file or directory`.
		return nil, operandFailure(path, err)
	}
	defer file.Close()
	return io.ReadAll(file)
}

func readOperandLines(ctx context.Context, view ProcessView, path string, stdin io.Reader) ([]string, error) {
	data, err := readOperand(ctx, view, path, stdin)
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(make([]byte, 0, 64*1024), maxTextLine)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}
