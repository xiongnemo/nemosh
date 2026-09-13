package applets

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"golang.org/x/term"
)

// less, the pager.
//
// **With nowhere to page to, it is `cat`.** `less file | head -3` and `less file > out` both
// copy their input through, because that is what every less does and what makes it safe to
// write in a script: the pager is a convenience for a person at a terminal, and a program
// downstream should never have to know one was in the way. It is also what makes this
// testable without a terminal at all.
//
// The whole of the interactive part is reading and scrolling. There is no editing, so the
// keys are the ones people already have in their fingers: space and `b` for pages, `j` and
// `k` for lines, `g` and `G` for the ends, `/` and `?` to search, `n` and `N` to repeat, and
// `q` to leave.

type lessOptions struct {
	numbers    bool
	chop       bool
	ignoreCase bool
	quitAtEOF  bool
	quitIfOne  bool
}

func newLessApplet() Applet {
	return simpleApplet{name: "less", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		// The options busybox takes. `-M`, `-m`, `-R` and `-~` are accepted and change
		// nothing: the first two pick between two prompt styles this has one of, and the
		// others describe drawing this does not do differently.
		options, operands, err := parseAppletOptions(args, "EFIMmNSRh~", "")
		if err != nil {
			return err
		}
		if options.has('h') {
			return writeLessHelp(stdout)
		}
		if len(operands) == 0 && lessInputIsTerminal(stdin) {
			// `less` with nothing to read would otherwise take the user's typing for the
			// file's contents and never show anything -- a hang, in the shape bc and dc
			// had. Every less refuses here instead.
			return fmt.Errorf("missing filename (`less -h` for help)")
		}
		lines, name, err := readLessInput(ctx, operands, stdin)
		if err != nil {
			return err
		}
		settings := lessOptions{
			numbers:    options.has('N'),
			chop:       options.has('S'),
			ignoreCase: options.has('I'),
			quitAtEOF:  options.has('E'),
			quitIfOne:  options.has('F'),
		}
		if !lessCanPage(stdout) {
			return writeLessPlainly(stdout, lines, settings)
		}
		screen, err := tcell.NewScreen()
		if err != nil {
			// A terminal that cannot be driven is not a failure worth stopping for: the
			// text is what was asked for, and the paging was the decoration.
			return writeLessPlainly(stdout, lines, settings)
		}
		return runLess(ctx, screen, lines, name, settings, stdout)
	}}
}

// lessInputIsTerminal reports whether standard input is a terminal rather than a file or a
// pipe -- which is to say, whether reading it would wait on a person.
func lessInputIsTerminal(stdin io.Reader) bool {
	file, ok := stdin.(*os.File)
	if !ok {
		// Wrapped by the shell, so it is not a terminal handle this can ask about.
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}

// lessCanPage reports whether there is a terminal to page on.
//
// **Standard output decides**, not standard input: `less file` with its input redirected is
// still a pager, and `less file | cat` is not. That is the way round every less has it, and
// the way round that makes a pipeline safe.
func lessCanPage(stdout io.Writer) bool {
	file, ok := stdout.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(file.Fd()))
}

// readLessInput gathers the text, from the operands or from standard input.
func readLessInput(ctx context.Context, operands []string, stdin io.Reader) ([]string, string, error) {
	if len(operands) == 0 {
		lines, err := readLessLines(decodeTextInput(stdin))
		return lines, "(standard input)", err
	}
	view := ProcessViewFromContext(ctx)
	var all []string
	for _, operand := range operands {
		if operand == "-" {
			lines, err := readLessLines(decodeTextInput(stdin))
			if err != nil {
				return nil, "", err
			}
			all = append(all, lines...)
			continue
		}
		native, err := resolveHostPath(view, operand)
		if err != nil {
			return nil, "", err
		}
		file, err := os.Open(native)
		if err != nil {
			return nil, "", cannotOpen(operand, err)
		}
		lines, err := readLessLines(decodeTextInput(file))
		file.Close()
		if err != nil {
			return nil, "", err
		}
		all = append(all, lines...)
	}
	name := operands[0]
	if len(operands) > 1 {
		name = fmt.Sprintf("%s (and %d more)", operands[0], len(operands)-1)
	}
	return all, name, nil
}

func readLessLines(reader io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), maxTextLine)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// writeLessPlainly is the `cat` path, with `-N` still honoured because a script may have
// asked for the numbers rather than for the paging.
func writeLessPlainly(out io.Writer, lines []string, settings lessOptions) error {
	writer := bufio.NewWriter(out)
	defer writer.Flush()
	for index, line := range lines {
		if settings.numbers {
			fmt.Fprintf(writer, "%6d  ", index+1)
		}
		if _, err := writer.WriteString(line + "\n"); err != nil {
			return err
		}
	}
	return nil
}

func writeLessHelp(out io.Writer) error {
	_, err := io.WriteString(out, strings.Join([]string{
		"less: page through text",
		"",
		"  SPACE, f, PgDn   forward one screen",
		"  b, PgUp          back one screen",
		"  j, DOWN, ENTER   forward one line",
		"  k, UP            back one line",
		"  g, HOME          the first line",
		"  G, END           the last line",
		"  /PATTERN         search forward",
		"  ?PATTERN         search backward",
		"  n, N             repeat the search, forward or back",
		"  -N               show line numbers",
		"  q                quit",
		"",
	}, "\n"))
	return err
}
