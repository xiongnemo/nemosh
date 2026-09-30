package applets

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
)

// catPrinter is how cat writes what it reads, as busybox's cat_main chooses (coreutils/cat.c).
// -v, -e, -t and -A, which is all three, show what does not print as catv does: ^X for a control
// character, M- before one with the high bit set, $ at each line's end under -e and ^I for a tab
// under -t, and a number there is six columns and two spaces. -n and -b alone number whole lines
// as print_numbered_lines does, six columns and a tab, -b passing over the empty ones. Nothing
// asked is the stream, byte for byte, which matters: cat is the applet a binary goes through.
//
// The count runs across the operands, so `cat -n a b` numbers one document. It numbered with a
// scanner, which failed on a line past 64K.
type catPrinter struct {
	visible, ends, tabs bool
	number, nonBlank    bool
	line                int
	// lineStart is catv's eol_seen: whether the next byte begins a line, across the operands.
	lineStart bool
}

func newCatPrinter(options appletOptions) *catPrinter {
	all := options.has('A')
	printer := &catPrinter{
		visible:  all || options.has('v') || options.has('e') || options.has('t'),
		ends:     all || options.has('e'),
		tabs:     all || options.has('t'),
		number:   options.has('n') || options.has('b'),
		nonBlank: options.has('b'),
	}
	printer.lineStart = printer.number
	return printer
}

func (p *catPrinter) copy(ctx context.Context, stdout io.Writer, input io.Reader) error {
	switch {
	case p.visible:
		return p.copyVisible(ctx, stdout, input)
	case p.number:
		return p.copyNumbered(ctx, stdout, input)
	}
	_, err := copyWithContext(ctx, stdout, input)
	return err
}

// copyNumbered is print_numbered_lines: each line, and a newline after it even when the last had
// none, after its number unless -b and it is empty. busybox-w32 reads these lines in msvcrt's
// text mode, which drops the carriage return before each newline, and so does this.
func (p *catPrinter) copyNumbered(ctx context.Context, stdout io.Writer, input io.Reader) error {
	reader := bufio.NewReader(contextReader{ctx: ctx, reader: input})
	writer := bufio.NewWriter(stdout)
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			text := line
			if strings.HasSuffix(line, "\n") {
				text = strings.TrimSuffix(line[:len(line)-1], "\r")
			}
			if !p.nonBlank || text != "" {
				p.line++
				fmt.Fprintf(writer, "%6d\t", p.line)
			}
			writer.WriteString(text)
			writer.WriteString("\n")
		}
		if err == io.EOF {
			return writer.Flush()
		}
		if err != nil {
			return joinFlush(writer, err)
		}
	}
}

// copyVisible is catv: byte by byte through busybox's visible(), a number before the first byte
// of each line under -n, and under -b before the first of each line that is not empty.
func (p *catPrinter) copyVisible(ctx context.Context, stdout io.Writer, input io.Reader) error {
	reader := bufio.NewReader(contextReader{ctx: ctx, reader: input})
	writer := bufio.NewWriter(stdout)
	for {
		c, err := reader.ReadByte()
		if err == io.EOF {
			return writer.Flush()
		}
		if err != nil {
			return joinFlush(writer, err)
		}
		if p.lineStart && !(p.nonBlank && c == '\n') {
			p.line++
			fmt.Fprintf(writer, "%6d  ", p.line)
		}
		p.lineStart = p.number && c == '\n'
		p.writeVisible(writer, c)
	}
}

// writeVisible is busybox's visible(): a tab as it is unless -t, a newline after $ under -e, and
// any other byte below 32 or 127 as ^ and the byte xor 0x40, after M- when its high bit is set.
func (p *catPrinter) writeVisible(writer *bufio.Writer, c byte) {
	switch {
	case c == '\t' && !p.tabs:
	case c == '\n':
		if p.ends {
			writer.WriteByte('$')
		}
	default:
		if c >= 128 {
			c -= 128
			writer.WriteString("M-")
		}
		if c < 32 || c == 127 {
			writer.WriteByte('^')
			c ^= 0x40
		}
	}
	writer.WriteByte(c)
}

// joinFlush flushes what was written before err, which is the one reported.
func joinFlush(writer *bufio.Writer, err error) error {
	writer.Flush()
	return err
}
