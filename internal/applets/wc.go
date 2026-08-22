package applets

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"unicode"

	"github.com/xiongnemo/nemosh/internal/textgrid"
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
		// More than one operand ends with a total, which both references print and this
		// did not: `wc a b` stopped after the second line, so a script adding up a
		// directory of files got no answer and no indication that it was missing.
		var total wcCounts
		for _, path := range paths {
			file, err := OpenProcessOperand(ctx, view, path, stdin)
			if err != nil {
				return operandFailure(path, err)
			}
			counts, countErr := countBytes(file)
			closeErr := file.Close()
			if err := errors.Join(countErr, closeErr); err != nil {
				return err
			}
			total = total.add(counts)
			if err := printWcCounts(stdout, flags, counts, path); err != nil {
				return err
			}
		}
		if len(paths) > 1 {
			return printWcCounts(stdout, flags, total, "total")
		}
		return nil
	}}
}

type wcFlags struct {
	lines bool
	words bool
	bytes bool
	// chars is -m, and it is not the same as -c. Measured on a two-line input
	// holding one accented letter: 19 bytes, 18 characters.
	//
	// GNU said 19 for both at first, which is a locale artifact rather than a
	// disagreement -- with no locale set a character *is* a byte. Under
	// LC_ALL=C.UTF-8 it says 18, and 18 is what this counts, because runes are
	// what everything else here measures in: the line editor's widths, `rev`,
	// `expr length`.
	chars bool
	// longest is -L, the width of the longest line, and width means *cells*: GNU
	// documents it as "the maximum display width" and 中文测试 is four characters
	// drawn in eight columns. This counted characters and answered 4, which is
	// neither reference's answer -- busybox says 3, having its own trouble with
	// multibyte input. The newline does not count.
	longest bool
}

type wcCounts struct {
	lines   int
	words   int
	bytes   int
	chars   int
	longest int
}

// wcColumnWidth is busybox-w32's field width for an aligned count.
const wcColumnWidth = 9

// add accumulates one file's counts into a running total. `-L` is the exception and it is
// busybox's: the longest line of several files is the longest of them, not their sum.
func (c wcCounts) add(other wcCounts) wcCounts {
	c.lines += other.lines
	c.words += other.words
	c.bytes += other.bytes
	c.chars += other.chars
	c.longest = max(c.longest, other.longest)
	return c
}

// An unknown letter used to be dropped on the floor after clearing the
// defaults, so `wc -z FILE` selected no counts at all and still exited 0 --
// printing a line with nothing on it but the filename.
func wcArgs(args []string) (wcFlags, []string, error) {
	options, paths, err := parseAppletOptions(args, "lwcmL", "")
	if err != nil {
		return wcFlags{}, nil, err
	}
	selected := wcFlags{
		lines:   options.has('l'),
		words:   options.has('w'),
		bytes:   options.has('c'),
		chars:   options.has('m'),
		longest: options.has('L'),
	}
	if !selected.lines && !selected.words && !selected.bytes && !selected.chars && !selected.longest {
		// The default three, and not the two that were added: GNU's default is
		// lines, words and bytes, and adding -m to it would change every existing
		// script's output.
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
	if flags.chars {
		values = append(values, counts.chars)
	}
	if flags.bytes {
		values = append(values, counts.bytes)
	}
	if flags.longest {
		values = append(values, counts.longest)
	}
	// Padded only when there is more than one count, which is the rule busybox-w32
	// applies: `wc -l f` prints `0 f` and `wc f` aligns its three columns. Measured,
	// because the references disagree on the width -- busybox pads to nine and GNU to
	// seven -- and the primary reference settles it.
	width := 0
	if len(values) > 1 {
		width = wcColumnWidth
	}
	for i, value := range values {
		if i > 0 {
			if _, err := fmt.Fprint(stdout, " "); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(stdout, "%*d", width, value); err != nil {
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
	lineWidth := 0
	for {
		r, size, err := reader.ReadRune()
		counts.bytes += size
		if size > 0 {
			counts.chars++
		}
		if r == '\n' {
			counts.lines++
			// -L is the width of the longest line, in characters, not counting
			// the newline.
			counts.longest = max(counts.longest, lineWidth)
			lineWidth = 0
		} else if size > 0 {
			lineWidth += textgrid.RuneCells(r)
		}
		if size > 0 && unicode.IsSpace(r) {
			inWord = false
		} else if size > 0 && !inWord {
			counts.words++
			inWord = true
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				// A final line without a newline is still a line for -L, which is
				// the width of the longest *line* rather than of the longest
				// newline-terminated one.
				counts.longest = max(counts.longest, lineWidth)
				return counts, nil
			}
			return wcCounts{}, err
		}
	}
}
