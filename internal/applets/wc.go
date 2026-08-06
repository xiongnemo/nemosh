package applets

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"unicode"
)

func newWcApplet() Applet {
	return simpleApplet{name: "wc", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		flags, paths := wcArgs(args)
		if len(paths) == 0 {
			counts, err := countBytes(stdin)
			if err != nil {
				return err
			}
			err = printWcCounts(stdout, flags, counts, "")
			return err
		}
		view := ProcessViewFromContext(ctx)
		for _, path := range paths {
			file, err := OpenProcessInput(ctx, view, path)
			if err != nil {
				return operandFailure(path, err)
			}
			counts, countErr := countBytes(file)
			closeErr := file.Close()
			if err := errors.Join(countErr, closeErr); err != nil {
				return err
			}
			if err := printWcCounts(stdout, flags, counts, path); err != nil {
				return err
			}
		}
		return nil
	}}
}

type wcFlags struct {
	lines bool
	words bool
	bytes bool
}

type wcCounts struct {
	lines int
	words int
	bytes int
}

func wcArgs(args []string) (wcFlags, []string) {
	flags := wcFlags{lines: true, words: true, bytes: true}
	pathsStart := 0
	seenFlag := false
	for pathsStart < len(args) && len(args[pathsStart]) > 1 && args[pathsStart][0] == '-' {
		for _, r := range args[pathsStart][1:] {
			if !seenFlag {
				flags = wcFlags{}
				seenFlag = true
			}
			switch r {
			case 'l':
				flags.lines = true
			case 'w':
				flags.words = true
			case 'c':
				flags.bytes = true
			}
		}
		pathsStart++
	}
	return flags, args[pathsStart:]
}

func printWcCounts(stdout io.Writer, flags wcFlags, counts wcCounts, path string) error {
	values := make([]int, 0, 3)
	if flags.lines {
		values = append(values, counts.lines)
	}
	if flags.words {
		values = append(values, counts.words)
	}
	if flags.bytes {
		values = append(values, counts.bytes)
	}
	for i, value := range values {
		if i > 0 {
			if _, err := fmt.Fprint(stdout, " "); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprint(stdout, value); err != nil {
			return err
		}
	}
	if path != "" {
		if _, err := fmt.Fprintf(stdout, " %s", path); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(stdout)
	return err
}

func countBytes(input io.Reader) (wcCounts, error) {
	reader := bufio.NewReader(input)
	counts := wcCounts{}
	inWord := false
	for {
		r, size, err := reader.ReadRune()
		counts.bytes += size
		if r == '\n' {
			counts.lines++
		}
		if size > 0 && unicode.IsSpace(r) {
			inWord = false
		} else if size > 0 && !inWord {
			counts.words++
			inWord = true
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return counts, nil
			}
			return wcCounts{}, err
		}
	}
}
