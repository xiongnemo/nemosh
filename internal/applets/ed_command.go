package applets

import (
	"bufio"
	"context"
	"fmt"
	"strings"
)

// ed's commands.
//
// Each takes the addresses that were written before it, and a default for when none were:
// `p` alone means `.p`, `a` means `.a`, and `w` means `1,$w`. Those defaults are most of what
// makes ed usable by hand, and all of what makes a script's `ed` depend on where `.` is.

func (b *edBuffer) command(ctx context.Context, line string, reader *bufio.Scanner) error {
	addresses, rest, err := b.parseAddresses(line)
	if err != nil {
		return err
	}
	rest = strings.TrimLeft(rest, " \t")
	if rest == "" {
		// An address with no command moves to it and prints it, which is how `5` on its
		// own works.
		return b.printAt(addresses)
	}
	name, argument := rest[0], strings.TrimLeft(rest[1:], " \t")
	switch name {
	case 'p', 'n', 'l':
		return b.printLines(addresses, name)
	case '=':
		return b.printNumber(addresses)
	case 'a', 'i':
		return b.appendLines(addresses, name, reader)
	case 'c':
		return b.changeLines(addresses, reader)
	case 'd':
		return b.deleteLines(addresses)
	case 'j':
		return b.joinLines(addresses)
	case 'm', 't':
		return b.moveLines(addresses, name, argument)
	case 'k':
		return b.markLine(addresses, argument)
	case 's':
		return b.substitute(addresses, rest[1:])
	case 'g', 'v':
		return b.global(ctx, addresses, name, rest[1:], reader)
	case 'w', 'W':
		return b.writeCommand(ctx, addresses, argument, name == 'W')
	case 'r':
		return b.readCommand(ctx, addresses, argument)
	case 'e', 'E':
		return b.editCommand(ctx, argument, name == 'E')
	case 'f':
		return b.fileCommand(argument)
	case 'q', 'Q':
		return b.quitCommand(name == 'Q')
	case 'h':
		if b.lastError != "" {
			fmt.Fprintln(b.out, b.lastError)
		}
		return nil
	case 'H':
		b.explaining = !b.explaining
		if b.explaining && b.lastError != "" {
			fmt.Fprintln(b.out, b.lastError)
		}
		return nil
	case 'P':
		// GNU toggles the prompt; with none set there is nothing to toggle to, so this
		// turns the default `*` on and off.
		if b.prompt == "" {
			b.prompt = "*"
		} else {
			b.prompt = ""
		}
		return nil
	case '#':
		// A comment: the rest of the line is ignored, which is how a script annotates.
		return nil
	}
	return fmt.Errorf("unknown command")
}

func (b *edBuffer) printAt(addresses edAddresses) error {
	if err := b.checkRange(addresses.last, addresses.last); err != nil {
		return err
	}
	b.setCurrent(addresses.last)
	b.writeLine(addresses.last, false, false)
	return nil
}

func (b *edBuffer) printLines(addresses edAddresses, form byte) error {
	if err := b.checkRange(addresses.first, addresses.last); err != nil {
		return err
	}
	for index := addresses.first; index <= addresses.last; index++ {
		b.writeLine(index, form == 'n', form == 'l')
	}
	b.setCurrent(addresses.last)
	return nil
}

// printNumber is `=`, which answers a line number rather than a line.
//
// With no address it is `$=`, the number of lines -- which is what makes `ed -s file <<< '='`
// a line count.
func (b *edBuffer) printNumber(addresses edAddresses) error {
	line := addresses.last
	if addresses.count == 0 {
		line = b.lineCount()
	}
	fmt.Fprintln(b.out, line)
	return nil
}

// appendLines is `a` and `i`, which differ only in which side of the line they land.
func (b *edBuffer) appendLines(addresses edAddresses, name byte, reader *bufio.Scanner) error {
	at := addresses.last
	if name == 'i' {
		// `i` puts the text *before* the line, so `1i` goes to the top.
		at--
		if at < 0 {
			at = 0
		}
	}
	if at > b.lineCount() {
		return fmt.Errorf("invalid address")
	}
	b.setCurrent(b.insertAt(at, b.readLines(reader)))
	return nil
}

func (b *edBuffer) changeLines(addresses edAddresses, reader *bufio.Scanner) error {
	if err := b.checkRange(addresses.first, addresses.last); err != nil {
		return err
	}
	b.deleteRange(addresses.first, addresses.last)
	b.setCurrent(b.insertAt(addresses.first-1, b.readLines(reader)))
	return nil
}

func (b *edBuffer) deleteLines(addresses edAddresses) error {
	if err := b.checkRange(addresses.first, addresses.last); err != nil {
		return err
	}
	b.deleteRange(addresses.first, addresses.last)
	// `.` becomes the line after what was removed, or the last line if there is none.
	b.setCurrent(addresses.first)
	if b.current > b.lineCount() {
		b.setCurrent(b.lineCount())
	}
	return nil
}

// joinLines is `j`: the addressed lines become one, with nothing between them.
func (b *edBuffer) joinLines(addresses edAddresses) error {
	first, last := addresses.first, addresses.last
	if addresses.count < 2 {
		// With one address `j` joins it with the next, which is the useful default.
		last = first + 1
	}
	if err := b.checkRange(first, last); err != nil {
		return err
	}
	if first == last {
		return nil
	}
	joined := strings.Join(b.lines[first-1:last], "")
	b.deleteRange(first, last)
	b.insertAt(first-1, []string{joined})
	b.setCurrent(first)
	return nil
}

// moveLines is `m` and `t`: move, and copy.
func (b *edBuffer) moveLines(addresses edAddresses, name byte, argument string) error {
	if err := b.checkRange(addresses.first, addresses.last); err != nil {
		return err
	}
	destination, _, found, err := b.parseOneAddress(argument)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("destination expected")
	}
	if destination < 0 || destination > b.lineCount() {
		return fmt.Errorf("invalid address")
	}
	if name == 'm' && destination >= addresses.first-1 && destination <= addresses.last {
		// Moving a range into itself cannot mean anything.
		return fmt.Errorf("invalid destination")
	}
	moved := append([]string{}, b.lines[addresses.first-1:addresses.last]...)
	if name == 'm' {
		b.deleteRange(addresses.first, addresses.last)
		if destination > addresses.last {
			destination -= addresses.last - addresses.first + 1
		}
	}
	b.setCurrent(b.insertAt(destination, moved))
	return nil
}

func (b *edBuffer) markLine(addresses edAddresses, argument string) error {
	if argument == "" {
		return fmt.Errorf("a mark needs a name")
	}
	if err := b.checkRange(addresses.last, addresses.last); err != nil {
		return err
	}
	b.marks[argument[0]] = addresses.last
	return nil
}

// quitCommand is `q`, which refuses once if there are unsaved changes, and `Q`, which does
// not.
//
// The refusal is the whole difference between them, and it is why `Q` exists at all.
func (b *edBuffer) quitCommand(force bool) error {
	if !force && b.dirty && !b.warned {
		b.warned = true
		return fmt.Errorf("warning: file modified")
	}
	b.quit = true
	return nil
}

func (b *edBuffer) fileCommand(argument string) error {
	if argument != "" {
		b.name = argument
		return nil
	}
	if b.name == "" {
		return fmt.Errorf("no current filename")
	}
	fmt.Fprintln(b.out, b.name)
	return nil
}
