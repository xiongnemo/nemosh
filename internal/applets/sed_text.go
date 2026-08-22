package applets

import "strings"

// The text commands: `a` appends after the line, `i` inserts before it, and `c`
// replaces it.
//
// Their argument is the one thing in sed that is not delimited, which is why they
// need their own reader: the text runs to the end of the line or the end of the
// script fragment, and a `;` inside it is text rather than a separator.
// Measured -- `sed '1a\x;p'` appends the literal `x;p`, where every other command
// would have taken `p` as the next one.

// parseSedTextCommand reads `a`, `i` or `c` and its text, returning what is left
// of the script.
func parseSedTextCommand(script string) (string, string) {
	rest := script[1:]
	if strings.HasPrefix(rest, `\`) {
		// A backslash introduces the text and protects its leading whitespace:
		// `a\  spaced` keeps both spaces, where `a  spaced` does not. It may be
		// followed by a newline, which is the POSIX spelling with the text on the
		// following line.
		rest = rest[1:]
		rest = strings.TrimPrefix(rest, "\n")
	} else {
		// Without one, leading blanks are separators rather than text.
		rest = strings.TrimLeft(rest, " \t")
	}
	text, remainder, found := strings.Cut(rest, "\n")
	if !found {
		remainder = ""
	}
	return unescapeSedText(text), remainder
}

// unescapeSedText interprets the escapes the text commands honour. `\n` and `\t`
// are what make a multi-line insert possible from one argument, since the shell
// has already eaten any real newline the user typed.
func unescapeSedText(text string) string {
	var out strings.Builder
	out.Grow(len(text))
	for index := 0; index < len(text); index++ {
		if text[index] != '\\' || index+1 >= len(text) {
			out.WriteByte(text[index])
			continue
		}
		index++
		switch text[index] {
		case 'n':
			out.WriteByte('\n')
		case 't':
			out.WriteByte('\t')
		case 'r':
			out.WriteByte('\r')
		case '\\':
			out.WriteByte('\\')
		default:
			// An unknown escape stands for the character itself, which is what
			// keeps `a\x\qy` from losing the q.
			out.WriteByte(text[index])
		}
	}
	return out.String()
}

// runSedTextCommand performs one of the three.
//
// `i` writes at once, even under -n, because it belongs before the pattern space
// whether or not the pattern space is printed.
//
// `a` is queued instead and flushed at the end of the cycle. That is not a
// convenience: the text belongs *after* the line, so it must outlive both the
// automatic print and a `d` that discards the line -- measured, `sed '1{a\X
// d}'` prints X and not the line.
//
// `c` replaces, and on a *range* it prints once at the end rather than per line:
// `sed '1,2c\once'` answers one `once`. That is why it consults the address.
func runSedTextCommand(command *sedCommand, cycle *sedCycle) (sedControl, error) {
	switch command.action {
	case 'i':
		return sedNext, cycle.write(command.text)
	case 'a':
		cycle.appended = append(cycle.appended, command.text)
		return sedNext, nil
	}
	// 'c': the pattern space is discarded either way, and the text is written
	// only as the address's run of lines ends.
	if command.address.atRangeEnd() {
		if err := cycle.write(command.text); err != nil {
			return sedDeleted, err
		}
	}
	return sedDeleted, nil
}

// flushAppended writes what `a` queued, after the pattern space and regardless of
// how the cycle ended.
func (c *sedCycle) flushAppended() error {
	for _, text := range c.appended {
		if err := c.write(text); err != nil {
			return err
		}
	}
	c.appended = nil
	return nil
}
