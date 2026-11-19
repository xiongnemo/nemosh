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
	// raw is -R, which draws colour sequences as colour, as GNU's does; see less_colour.go.
	raw bool
}

func newLessApplet() Applet {
	return simpleApplet{name: "less", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		// The options busybox takes. `-M`, `-m` and `-~` are accepted and change nothing:
		// the first two pick between two prompt styles this has one of, and the last
		// describes drawing this does not do differently.
		options, operands, err := parseAppletOptions(ctx, args, "EFIMmNSRh~", "")
		if err != nil {
			return err
		}
		if options.has('h') {
			return writeLessHelp(stdout)
		}
		if len(operands) == 0 && inputIsTerminal(ctx, stdin) {
			// `less` with nothing to read would otherwise take the user's typing for the
			// file's contents and never show anything -- a hang, in the shape bc and dc
			// had. Every less refuses here instead.
			return fmt.Errorf("missing filename (`less -h` for help)")
		}
		sources, name, err := openLessSources(ctx, operands, stdin)
		if err != nil {
			return err
		}
		settings := lessOptions{
			numbers:    options.has('N'),
			chop:       options.has('S'),
			ignoreCase: options.has('I'),
			quitAtEOF:  options.has('E'),
			quitIfOne:  options.has('F'),
			raw:        options.has('R'),
		}
		var screen tcell.Screen
		if lessCanPage(stdout) {
			// A terminal that cannot be driven is not a failure worth stopping for: the
			// text is what was asked for, and the paging was the decoration.
			screen, _ = tcell.NewScreen()
		}
		if screen == nil {
			return writeLessStreaming(ctx, stdout, sources, settings)
		}
		return runLess(ctx, screen, startLessFeed(ctx, sources), name, settings, stdout)
	}}
}

// inputIsTerminal reports whether standard input is a terminal rather than a file or a
// pipe -- which is to say, whether reading it would wait on a person. less asks it before
// paging nothing, and rm before asking about a file it cannot write.
//
// Asked of the stream rather than of the value: the shell hands an applet a reader that
// forwards to the console, never the console itself, so `stdin.(*os.File)` answers no
// inside the shell and `less` with no operands read the keyboard as though it were a file
// instead of refusing. `top` has asked this way since the same mistake stopped it drawing.
//
// The lease is taken and given straight back, because this is a question rather than a use.
func inputIsTerminal(ctx context.Context, stdin io.Reader) bool {
	file, release, ok := leaseTopStdin(ctx, stdin)
	if !ok || file == nil {
		return false
	}
	release()
	return term.IsTerminal(int(file.Fd()))
}

// lessCanPage reports whether there is a terminal to page on.
//
// **Standard output decides**, not standard input: `less file` with its input redirected is
// still a pager, and `less file | cat` is not. That is the way round every less has it, and
// the way round that makes a pipeline safe.
//
// stdoutIsTerminal walks the chain of wrappers to the file at the end of it. This used to
// ask `stdout.(*os.File)` directly, which is always false inside the shell -- everything an
// applet writes to goes through a descriptorWriter -- so `less` decided there was no
// terminal every single time it was run from a prompt, and printed the file instead of
// paging it. See fd_stream.go, which exists for this question.
func lessCanPage(stdout io.Writer) bool {
	return stdoutIsTerminal(stdout)
}

// openLessSources opens what is to be read, the operands or standard input, before anything
// is read: a file that cannot be opened stops less with the reason, as it did when the reading
// came first, rather than turning up partway through the paging.
func openLessSources(ctx context.Context, operands []string, stdin io.Reader) ([]lessSource, string, error) {
	if len(operands) == 0 {
		return []lessSource{{reader: stdin}}, "(standard input)", nil
	}
	view := ProcessViewFromContext(ctx)
	var sources []lessSource
	for _, operand := range operands {
		if operand == "-" {
			sources = append(sources, lessSource{reader: stdin})
			continue
		}
		native, err := resolveHostPath(view, operand)
		if err == nil {
			var file *os.File
			if file, err = os.Open(native); err == nil {
				sources = append(sources, lessSource{reader: file, close: func() { file.Close() }})
				continue
			}
			err = cannotOpen(operand, err)
		}
		for _, opened := range sources {
			if opened.close != nil {
				opened.close()
			}
		}
		return nil, "", err
	}
	name := operands[0]
	if len(operands) > 1 {
		name = fmt.Sprintf("%s (and %d more)", operands[0], len(operands)-1)
	}
	return sources, name, nil
}

// writeLessStreaming is the `cat` path, a line written when it is read and flushed when the
// input goes quiet, as the text filters flush (filter_output.go): it read everything first, so
// `tail -f log | less > out` wrote nothing. -N is still honoured, because a script may have
// asked for the numbers rather than for the paging. What it reads it writes as it is, colour
// sequences too, as GNU's less copies them; busybox's -R took them out here as well.
func writeLessStreaming(ctx context.Context, stdout io.Writer, sources []lessSource, settings lessOptions) error {
	output := newFilterOutput(stdout)
	number := 0
	var failure error
	for _, source := range sources {
		if failure == nil {
			failure = readLessSource(ctx, output.input(contextReader{ctx: ctx, reader: source.reader}), func(line string) {
				number++
				if settings.numbers {
					fmt.Fprintf(output, "%6d  ", number)
				}
				output.WriteString(line + "\n")
			})
		}
		if source.close != nil {
			source.close()
		}
	}
	if err := output.Flush(); failure == nil {
		failure = err
	}
	return failure
}

// writeLessPlainly writes lines already read, for -F, whose text turned out to fit.
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
