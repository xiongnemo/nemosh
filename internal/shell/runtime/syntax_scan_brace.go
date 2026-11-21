package runtime

// braceParameterMarker stands on the quote stack for an unquoted `${` that does not close on
// its line. The newline after it is the expansion's, as both references read it: `echo H${$+`
// then `}H` is H H, and `${u:-` then `fn}` is fn. The line ended there, and the script was
// refused as missing a `}`. What follows is scanned as quoted, so a # in it is no comment and
// a ( opens no group, except that a quote opens as it does outside one, and its `}` closes it.
const braceParameterMarker byte = 2

// quotedBraceParameterMarker is the same for a `${` inside double quotes, where a double quote
// in the word still opens one but a single quote is a character, as bash and busybox keep it:
// `"${X-'a b'}"` is 'a b' with its quotes. a="${X-"one<newline>two"}" is one, a newline and
// two in both; the inner quote was taken for the outer one's end, the line ended with it, and
// the script was refused as missing a `}` -- fzf's completion.bash among them.
const quotedBraceParameterMarker byte = 3

// bare is whether the scan stands where a quote may open: outside every quote, or inside an
// unquoted `${` that goes on past its line.
func (scanner *syntaxScanner) bare() bool {
	return scanner.quote() == 0 || scanner.quote() == braceParameterMarker
}

// inBraceParameter is whether the scan stands inside a `${` that goes on past its line,
// quoted or not.
func (scanner *syntaxScanner) inBraceParameter() bool {
	return scanner.quote() == braceParameterMarker || scanner.quote() == quotedBraceParameterMarker
}

// openBraceParameter is the `${` at index that does not close on line, when the scan stands
// bare or inside double quotes: it opens the marker, and the scan goes on after the brace.
func (scanner *syntaxScanner) openBraceParameter(line string, index int) (int, bool) {
	switch {
	case scanner.bare():
		scanner.quotes = append(scanner.quotes, braceParameterMarker)
	case scanner.quote() == '"' || scanner.quote() == quotedBraceParameterMarker:
		scanner.quotes = append(scanner.quotes, quotedBraceParameterMarker)
	default:
		return index, false
	}
	scanner.logical.WriteString(line[index : index+2])
	return index + 1, true
}
