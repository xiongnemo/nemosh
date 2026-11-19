package applets

import (
	"bufio"
	"context"
	"errors"
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
		out := bufio.NewWriterSize(stdout, 32<<10)
		left, err := openOperandLines(ctx, view, paths[0], stdin, out)
		if err != nil {
			return err
		}
		defer left.close()
		// `comm - -`: the first takes all of standard input, and the second finds it ended.
		rightStdin := stdin
		if paths[0] == "-" && paths[1] == "-" {
			rightStdin = strings.NewReader("")
		}
		right, err := openOperandLines(ctx, view, paths[1], rightStdin, out)
		if err != nil {
			return err
		}
		defer right.close()
		show := [3]bool{!options.has('1'), !options.has('2'), !options.has('3')}
		if err := writeCommColumns(out, left, right, show); err != nil {
			return err
		}
		return errors.Join(out.Flush(), left.scanner.Err(), right.scanner.Err())
	}}
}

func writeCommColumns(stdout io.Writer, left, right *operandLines, show [3]bool) error {
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
	l, leftOK := left.next()
	r, rightOK := right.next()
	for leftOK && rightOK {
		switch {
		case l < r:
			if err := emit(0, l); err != nil {
				return err
			}
			l, leftOK = left.next()
		case l > r:
			if err := emit(1, r); err != nil {
				return err
			}
			r, rightOK = right.next()
		default:
			if err := emit(2, l); err != nil {
				return err
			}
			l, leftOK = left.next()
			r, rightOK = right.next()
		}
	}
	for ; leftOK; l, leftOK = left.next() {
		if err := emit(0, l); err != nil {
			return err
		}
	}
	for ; rightOK; r, rightOK = right.next() {
		if err := emit(1, r); err != nil {
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
		if len(paths) == 0 {
			paths = []string{"-"}
		}
		out := bufio.NewWriterSize(stdout, 32<<10)
		sources, err := openPasteSources(ctx, ProcessViewFromContext(ctx), paths, stdin, out)
		defer func() {
			for _, source := range sources {
				source.close()
			}
		}()
		if err != nil {
			return err
		}
		if options.has('s') {
			err = pasteSerially(out, sources, delimiters)
		} else {
			err = pasteInParallel(out, sources, delimiters)
		}
		if err != nil {
			return err
		}
		errs := []error{out.Flush()}
		for _, source := range sources {
			errs = append(errs, source.scanner.Err())
		}
		return errors.Join(errs...)
	}}
}

// openPasteSources opens each operand's lines, in order.
//
// Every `-` is the one standard input, read a line to a column in turn, as busybox and POSIX
// read it: `seq 5 | paste - -` is 1 2, 3 4 and 5. The first `-` took the whole input, so each line
// came out alone. -s reads each operand to its end, and so a second `-` finds nothing left there
// too. One source of standard input serves every `-`, which is both.
func openPasteSources(ctx context.Context, view ProcessView, paths []string, stdin io.Reader, out *bufio.Writer) ([]*operandLines, error) {
	sources := make([]*operandLines, 0, len(paths))
	var shared *operandLines
	for _, path := range paths {
		if path == "-" && shared != nil {
			sources = append(sources, shared)
			continue
		}
		source, err := openOperandLines(ctx, view, path, stdin, out)
		if err != nil {
			return sources, err
		}
		if path == "-" {
			shared = source
		}
		sources = append(sources, source)
	}
	return sources, nil
}

// pasteInParallel writes a line of every source a row, until none has one.
func pasteInParallel(out io.Writer, sources []*operandLines, delimiters []rune) error {
	fields := make([]string, len(sources))
	for {
		any := false
		for index, source := range sources {
			line, ok := source.next()
			fields[index], any = line, any || ok
		}
		if !any {
			return nil
		}
		if _, err := io.WriteString(out, joinWithDelimiters(fields, delimiters)+"\n"); err != nil {
			return err
		}
	}
}

// pasteSerially writes each source's lines as one, between the delimiters in turn. One with no
// lines writes none, as busybox's paste_files_separate writes none, where this wrote an empty one.
func pasteSerially(out io.Writer, sources []*operandLines, delimiters []rune) error {
	for _, source := range sources {
		line, ok := source.next()
		if !ok {
			continue
		}
		var err error
		for gap := 0; ok && err == nil; gap++ {
			if gap > 0 {
				if delimiter := delimiters[(gap-1)%len(delimiters)]; delimiter != 0 {
					_, err = io.WriteString(out, string(delimiter))
				}
			}
			if err == nil {
				_, err = io.WriteString(out, line)
			}
			line, ok = source.next()
		}
		if err == nil {
			_, err = io.WriteString(out, "\n")
		}
		if err != nil {
			return err
		}
	}
	return nil
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
