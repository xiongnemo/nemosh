package applets

import (
	"fmt"
	"io"
	"strings"
)

// Running the script over one line.
//
// Split out when `{}` blocks arrived, because a block turns a flat walk over the
// commands into a recursive one -- and `d` and `q` inside a block have to end the
// whole cycle rather than just the block. A boolean return could not say which of
// three things happened, so the walk reports a control signal.

// sedControl is what a command did to the flow of the cycle.
type sedControl byte

const (
	// sedNext carries on with the following command.
	sedNext sedControl = iota
	// sedDeleted is `d`: the pattern space is discarded, the rest of the script
	// is skipped, and there is no automatic print. This is what makes
	// `sed '2d;s/a/b/'` leave line two alone rather than substituting into a
	// line it dropped.
	sedDeleted
	// sedQuit is `q`: as above, and no further line is read.
	sedQuit
)

// sedCycle is the state one line is processed with.
type sedCycle struct {
	line   string
	number int
	isLast bool
	stdout io.Writer
	quiet  bool
	// printed records whether anything was written for this line, so `q` can
	// avoid printing the pattern space twice.
	printed bool
	// appended holds what `a` queued, written at the end of the cycle.
	appended []string
}

// runSedCommands walks a command list, recursing into blocks.
func runSedCommands(commands []*sedCommand, cycle *sedCycle) (sedControl, error) {
	for _, command := range commands {
		if !command.address.selects(cycle.line, cycle.number, cycle.isLast) {
			continue
		}
		control, err := runSedCommand(command, cycle)
		if err != nil || control != sedNext {
			return control, err
		}
	}
	return sedNext, nil
}

func runSedCommand(command *sedCommand, cycle *sedCycle) (sedControl, error) {
	switch command.action {
	case '{':
		// A block runs its own list under this command's address, which is the
		// whole point: `/x/{p;q}` applies both to the matching line only.
		return runSedCommands(command.block, cycle)
	case 's':
		cycle.line = command.substitute.replace(cycle.line)
	case 'y':
		cycle.line = command.translate.apply(cycle.line)
	case 'p':
		return sedNext, cycle.write(cycle.line)
	case '=':
		// The line number, on a line of its own, before the line itself.
		return sedNext, cycle.write(fmt.Sprintf("%d", cycle.number))
	case 'a', 'i', 'c':
		return runSedTextCommand(command, cycle)
	case 'd':
		return sedDeleted, nil
	case 'q':
		// The pattern space is still printed unless -n, then the run ends.
		if cycle.quiet {
			return sedQuit, nil
		}
		return sedQuit, cycle.write(cycle.line)
	}
	return sedNext, nil
}

func (c *sedCycle) write(text string) error {
	if _, err := fmt.Fprintln(c.stdout, text); err != nil {
		return err
	}
	c.printed = true
	return nil
}

// sedTranslate is the `y/from/to/` table.
//
// A map rather than a 256-byte array because the operands are text: `y/áé/ae/`
// should transliterate runes, and indexing bytes would corrupt the input by
// replacing half of one.
type sedTranslate struct{ pairs map[rune]rune }

func (t sedTranslate) apply(line string) string {
	var out strings.Builder
	out.Grow(len(line))
	for _, character := range line {
		if replacement, ok := t.pairs[character]; ok {
			out.WriteRune(replacement)
			continue
		}
		out.WriteRune(character)
	}
	return out.String()
}

// parseSedTranslateCommand reads one `y/from/to/`.
//
// Unequal lengths are refused. busybox instead transliterates the pairs it has
// and silently ignores the rest -- `y/abc/xy/` leaves every c alone, measured --
// which is a wrong answer with no diagnostic, the shape this project hunts. GNU
// refuses it, saying the strings are different lengths, and so does this.
func parseSedTranslateCommand(script string) (sedTranslate, string, error) {
	if len(script) < 3 {
		return sedTranslate{}, "", fmt.Errorf("unterminated `y' command")
	}
	delimiter := script[1]
	from, rest, err := readSedDelimited(script[2:], delimiter)
	if err != nil {
		return sedTranslate{}, "", fmt.Errorf("unterminated `y' command")
	}
	to, rest, err := readSedDelimited(rest, delimiter)
	if err != nil {
		return sedTranslate{}, "", fmt.Errorf("unterminated `y' command")
	}
	sources, targets := []rune(from), []rune(to)
	if len(sources) != len(targets) {
		return sedTranslate{}, "", fmt.Errorf("strings for `y' command are different lengths")
	}
	pairs := make(map[rune]rune, len(sources))
	for index, source := range sources {
		pairs[source] = targets[index]
	}
	return sedTranslate{pairs: pairs}, rest, nil
}
