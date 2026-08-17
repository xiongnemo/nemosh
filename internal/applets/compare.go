package applets

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
)

// cmp, comm and paste: the three that take two inputs at once.
//
// Measured against GNU coreutils, which is what these names mean to anyone who
// types them.

// cmp reports the first byte where two files differ.
//
// GNU's wording, measured:
//
//	$ cmp c1 c2
//	c1 c2 differ: char 3, line 1
//	$ echo $?
//	1
//
// One-based, both counts, and the message goes to stdout rather than stderr --
// which is surprising, and is GNU's behaviour. `-s` says nothing at all and
// leaves only the status, which is how a script uses it.
func newCmpApplet() Applet {
	return simpleApplet{name: "cmp", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, paths, err := parseAppletOptions(args, "sl", "")
		if err != nil {
			return err
		}
		if len(paths) != 2 {
			return fmt.Errorf("expected two operands, got %d", len(paths))
		}
		view := ProcessViewFromContext(ctx)
		contents := make([][]byte, 2)
		for index, path := range paths {
			data, err := readOperand(ctx, view, path, stdin)
			if err != nil {
				return err
			}
			contents[index] = data
		}
		offset, line, differ := firstDifference(contents[0], contents[1])
		if !differ {
			return nil
		}
		if options.has('s') {
			return ExitStatus(1)
		}
		if len(contents[0]) != len(contents[1]) && offset == min(len(contents[0]), len(contents[1])) {
			// One is a prefix of the other. GNU says so rather than naming a
			// byte that does not exist in the shorter file.
			shorter := paths[0]
			if len(contents[1]) < len(contents[0]) {
				shorter = paths[1]
			}
			fmt.Fprintf(stdout, "cmp: EOF on %s\n", shorter)
			return ExitStatus(1)
		}
		fmt.Fprintf(stdout, "%s %s differ: char %d, line %d\n", paths[0], paths[1], offset+1, line)
		return ExitStatus(1)
	}}
}

// firstDifference returns the byte offset, the one-based line it falls on, and
// whether there was one at all.
func firstDifference(left, right []byte) (int, int, bool) {
	line := 1
	for index := 0; index < len(left) && index < len(right); index++ {
		if left[index] != right[index] {
			return index, line, true
		}
		if left[index] == '\n' {
			line++
		}
	}
	if len(left) == len(right) {
		return 0, 0, false
	}
	return min(len(left), len(right)), line, true
}

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
		options, paths, err := parseAppletOptions(args, "123", "")
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
		options, paths, err := parseAppletOptions(args, "s", "d")
		if err != nil {
			return err
		}
		delimiters := "\t"
		if options.has('d') {
			if delimiters = options.value('d'); delimiters == "" {
				return fmt.Errorf("delimiter list cannot be empty")
			}
		}
		view := ProcessViewFromContext(ctx)
		columns := make([][]string, 0, max(len(paths), 1))
		if len(paths) == 0 {
			paths = []string{"-"}
		}
		for _, path := range paths {
			lines, err := readOperandLines(ctx, view, path, stdin)
			if err != nil {
				return err
			}
			columns = append(columns, lines)
		}
		if options.has('s') {
			for _, lines := range columns {
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

// joinWithDelimiters cycles through the delimiter list, which is what -d takes:
// `-d,;` alternates comma and semicolon between columns.
func joinWithDelimiters(fields []string, delimiters string) string {
	separators := []rune(delimiters)
	var joined strings.Builder
	for index, field := range fields {
		if index > 0 {
			joined.WriteRune(separators[(index-1)%len(separators)])
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
		return nil, cannotOpen(path, err)
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
