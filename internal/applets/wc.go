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
		flags, paths, err := wcArgs(args)
		if err != nil {
			return err
		}
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

// An unknown letter used to be dropped on the floor after clearing the
// defaults, so `wc -z FILE` selected no counts at all and still exited 0 --
// printing a line with nothing on it but the filename.
func wcArgs(args []string) (wcFlags, []string, error) {
	options, paths, err := parseAppletOptions(args, "lwc", "")
	if err != nil {
		return wcFlags{}, nil, err
	}
	selected := wcFlags{lines: options.has('l'), words: options.has('w'), bytes: options.has('c')}
	if !selected.lines && !selected.words && !selected.bytes {
		selected = wcFlags{lines: true, words: true, bytes: true}
	}
	return selected, paths, nil
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
