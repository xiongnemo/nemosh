package applets

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
)

// ed's file commands: `r`, `w`, `e` and the byte counts they print.
//
// **The byte count is output, not decoration.** `ed -s` suppresses it precisely because a
// script driving ed reads what ed prints, and a stray number on standard output would be
// taken for an answer. Without `-s` every read and every write reports how many bytes
// moved, which is what a person at the keyboard wants.
//
// A file is read through the same UTF-16 decoding every text applet uses, and written back
// as UTF-8 with the line ending this shell writes -- so editing a file Notepad made does not
// leave interleaved NULs behind.

// readInto loads a file after line `at`, reporting its size unless `-s` said not to.
func (b *edBuffer) readInto(view ProcessView, name string, at int, replacing bool) error {
	native, err := resolveHostPath(view, name)
	if err != nil {
		return err
	}
	file, err := os.Open(native)
	if err != nil {
		if replacing {
			// A file that is not there yet is the normal way to start one, so this is
			// not a failure -- the buffer is simply empty.
			if os.IsNotExist(err) {
				b.report(0)
				return nil
			}
		}
		return err
	}
	defer file.Close()
	lines, size, err := readEdLines(file)
	if err != nil {
		return err
	}
	if replacing {
		b.lines = lines
		b.setCurrent(b.lineCount())
		b.dirty = false
		b.warned = false
	} else {
		b.setCurrent(b.insertAt(at, lines))
	}
	b.report(size)
	return nil
}

func readEdLines(file *os.File) ([]string, int, error) {
	reader := bufio.NewScanner(decodeTextInput(file))
	reader.Buffer(make([]byte, 0, 64*1024), maxTextLine)
	var lines []string
	size := 0
	for reader.Scan() {
		lines = append(lines, reader.Text())
		// The newline counts, which is what makes the reported size match the file.
		size += len(reader.Text()) + 1
	}
	return lines, size, reader.Err()
}

// report prints a byte count, unless -s asked for silence.
func (b *edBuffer) report(size int) {
	if b.silent {
		return
	}
	fmt.Fprintln(b.out, size)
}

// writeCommand is `w` and `W`, which differ only in whether they append.
func (b *edBuffer) writeCommand(ctx context.Context, addresses edAddresses, name string, appending bool) error {
	first, last := addresses.first, addresses.last
	if addresses.count == 0 {
		// `w` with no address writes the whole buffer, which is what makes it safe to
		// type after a `p` that moved `.`.
		first, last = 1, b.lineCount()
	}
	if b.lineCount() == 0 {
		first, last = 1, 0
	} else if err := b.checkRange(first, last); err != nil {
		return err
	}
	target := strings.TrimSpace(name)
	if target == "" {
		target = b.name
	}
	if target == "" {
		return fmt.Errorf("no current filename")
	}
	b.name = target
	native, err := resolveHostPath(ProcessViewFromContext(ctx), target)
	if err != nil {
		return err
	}
	size, err := writeEdLines(native, b.lines[first-1:last], appending)
	if err != nil {
		return err
	}
	b.report(size)
	// Only a whole-buffer write clears the flag: having written three lines out is not
	// the same as having saved the file, and `q` should still refuse.
	if first == 1 && last == b.lineCount() {
		b.dirty = false
		b.warned = false
	}
	return nil
}

func writeEdLines(native string, lines []string, appending bool) (int, error) {
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if appending {
		flags = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	}
	file, err := os.OpenFile(native, flags, 0o644)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	size := 0
	for _, line := range lines {
		if _, err := writer.WriteString(line + "\n"); err != nil {
			return 0, err
		}
		size += len(line) + 1
	}
	return size, writer.Flush()
}

// readCommand is `r`, which inserts a file after the addressed line.
func (b *edBuffer) readCommand(ctx context.Context, addresses edAddresses, name string) error {
	target := strings.TrimSpace(name)
	if target == "" {
		target = b.name
	}
	if target == "" {
		return fmt.Errorf("no current filename")
	}
	at := addresses.last
	if addresses.count == 0 {
		// `r` with no address reads at the end, which is the useful default.
		at = b.lineCount()
	}
	if at < 0 || at > b.lineCount() {
		return fmt.Errorf("invalid address")
	}
	if err := b.readInto(ProcessViewFromContext(ctx), target, at, false); err != nil {
		return fmt.Errorf("cannot open %s", target)
	}
	return nil
}

// editCommand is `e`, which replaces the buffer, and `E`, which does so without asking.
func (b *edBuffer) editCommand(ctx context.Context, name string, force bool) error {
	if b.dirty && !force && !b.warned {
		b.warned = true
		return fmt.Errorf("warning: file modified")
	}
	target := strings.TrimSpace(name)
	if target == "" {
		target = b.name
	}
	if target == "" {
		return fmt.Errorf("no current filename")
	}
	b.name = target
	if err := b.readInto(ProcessViewFromContext(ctx), target, 0, true); err != nil {
		return fmt.Errorf("cannot open %s", target)
	}
	return nil
}
