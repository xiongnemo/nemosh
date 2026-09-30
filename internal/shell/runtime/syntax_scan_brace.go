package runtime

// braceParameterMarker stands on the quote stack for an unquoted `${` that does not close on
// its line. The newline after it is the expansion's, as both references read it: `echo H${$+`
// then `}H` is H H, and `${u:-` then `fn}` is fn. The line ended there, and the script was
// refused as missing a `}`. What follows is scanned as quoted, so a # in it is no comment and
// a ( opens no group, except that a quote opens as it does outside one, and its `}` closes it.
const braceParameterMarker byte = 2

// bare is whether the scan stands where a quote may open: outside every quote, or inside an
// unquoted `${` that goes on past its line.
func (scanner *syntaxScanner) bare() bool {
	return scanner.quote() == 0 || scanner.quote() == braceParameterMarker
}

// openBraceParameter is the `${` at index that does not close on line, when the scan stands
// bare: it opens the marker, and the scan goes on after the brace.
func (scanner *syntaxScanner) openBraceParameter(line string, index int) (int, bool) {
	if !scanner.bare() {
		return index, false
	}
	scanner.quotes = append(scanner.quotes, braceParameterMarker)
	scanner.logical.WriteString(line[index : index+2])
	return index + 1, true
}
