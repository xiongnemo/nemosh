package applets

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
)

// ed, the line editor.
//
// It is here because it is the editor a *script* can drive: `nano` and `micro` need a
// terminal, and a shell script that has to change three lines of a file has nothing else to
// reach for on Windows.
//
// **This follows POSIX and GNU rather than busybox-w32**, which is the one place in this
// project that happens, and it is deliberate: busybox's ed answers `unimplemented command`
// to `n`, cannot search backwards, and does not wrap a forward search -- so a differential
// test against it would pin down a subset rather than the language. Where the two can be
// compared they agree, and the cases that busybox cannot answer say so.
//
// The error convention is ed's own and looks unhelpful on purpose: a failure prints `?` and
// nothing else, and `h` then explains it. That is not obstinacy for its own sake -- ed is
// driven by scripts that check the output, and a diagnostic that changed wording between
// versions would break them.

type edBuffer struct {
	lines []string
	// current is ed's `.`, one-based. Zero means the buffer is empty.
	current int
	marks   map[byte]int
	// dirty says whether anything has changed since the last write, which is what makes
	// `q` refuse the first time and accept the second.
	dirty bool
	name  string
	// lastError is what `h` explains and `H` prints as it happens.
	lastError  string
	explaining bool
	// lastPattern is the empty `//`, which repeats the previous search.
	lastPattern string
	silent      bool
	prompt      string
	out         *bufio.Writer
	errors      io.Writer
	quit        bool
	// warned records that `q` has already refused once.
	warned bool
}

func newEdApplet() Applet {
	return simpleApplet{name: "ed", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		options, operands, err := parseAppletOptions(args, "s", "p")
		if err != nil {
			return err
		}
		if len(operands) > 1 {
			return fmt.Errorf("extra operand '%s'", operands[1])
		}
		buffer := &edBuffer{
			marks:  map[byte]int{},
			silent: options.has('s'),
			prompt: options.values['p'],
			out:    bufio.NewWriter(stdout),
			errors: stderr,
		}
		defer buffer.out.Flush()
		if len(operands) == 1 {
			// A file named on the command line is read at once, and its size reported
			// exactly as `e` would report it.
			buffer.name = operands[0]
			if err := buffer.readInto(ProcessViewFromContext(ctx), operands[0], 0, true); err != nil {
				fmt.Fprintf(stderr, "%s: %s\n", operands[0], CauseText(err))
			}
		}
		return buffer.run(ctx, stdin)
	}}
}

// run is the command loop.
func (b *edBuffer) run(ctx context.Context, stdin io.Reader) error {
	reader := bufio.NewScanner(decodeTextInput(stdin))
	reader.Buffer(make([]byte, 0, 64*1024), maxTextLine)
	for !b.quit {
		if err := ctx.Err(); err != nil {
			return err
		}
		if b.prompt != "" {
			b.out.WriteString(b.prompt)
			b.out.Flush()
		}
		if !reader.Scan() {
			return reader.Err()
		}
		if err := b.command(ctx, reader.Text(), reader); err != nil {
			b.fail(err)
		}
		b.out.Flush()
	}
	return nil
}

// fail is ed's whole error convention: a `?`, and the reason kept for `h`.
func (b *edBuffer) fail(err error) {
	b.lastError = err.Error()
	b.out.WriteString("?\n")
	if b.explaining {
		// `H` asks for the explanation as it happens rather than on request.
		fmt.Fprintf(b.out, "%s\n", b.lastError)
	}
}

// lineCount is `$`.
func (b *edBuffer) lineCount() int { return len(b.lines) }

// checkRange refuses an address outside the buffer, which is most of ed's errors.
func (b *edBuffer) checkRange(first, last int) error {
	if first < 1 || last > b.lineCount() || first > last {
		return fmt.Errorf("invalid address")
	}
	return nil
}

// setCurrent moves `.`, clamped to the buffer.
func (b *edBuffer) setCurrent(line int) {
	if line < 0 {
		line = 0
	}
	if line > b.lineCount() {
		line = b.lineCount()
	}
	b.current = line
}

// readLines gathers input up to a lone `.`, which is how `a`, `i` and `c` take their text.
func (b *edBuffer) readLines(reader *bufio.Scanner) []string {
	var gathered []string
	for reader.Scan() {
		line := reader.Text()
		if line == "." {
			return gathered
		}
		gathered = append(gathered, line)
	}
	// End of input ends the text, which is what makes `printf 'a\nx\n' | ed` work without
	// the terminating dot.
	return gathered
}

// insertAt puts lines after `at`, and answers where the last of them landed.
func (b *edBuffer) insertAt(at int, lines []string) int {
	if len(lines) == 0 {
		return at
	}
	tail := append([]string{}, b.lines[at:]...)
	b.lines = append(b.lines[:at], append(append([]string{}, lines...), tail...)...)
	b.dirty = true
	b.warned = false
	return at + len(lines)
}

// deleteRange removes lines first..last inclusive.
func (b *edBuffer) deleteRange(first, last int) []string {
	removed := append([]string{}, b.lines[first-1:last]...)
	b.lines = append(b.lines[:first-1], b.lines[last:]...)
	b.dirty = true
	b.warned = false
	b.forgetMarks(first, last)
	return removed
}

// forgetMarks drops the marks that pointed into deleted lines.
func (b *edBuffer) forgetMarks(first, last int) {
	for name, line := range b.marks {
		if line >= first && line <= last {
			delete(b.marks, name)
		}
	}
}

// writeLine prints one line, in the form `p`, `n` or `l` asked for.
func (b *edBuffer) writeLine(index int, numbered, visible bool) {
	if numbered {
		fmt.Fprintf(b.out, "%d\t", index)
	}
	text := b.lines[index-1]
	if visible {
		// `l` makes the invisible visible: a `$` marks the end of the line, and a tab is
		// written as the two characters that produced it.
		text = strings.ReplaceAll(text, "\\", "\\\\")
		text = strings.ReplaceAll(text, "\t", "\\t")
		text += "$"
	}
	fmt.Fprintln(b.out, text)
}
