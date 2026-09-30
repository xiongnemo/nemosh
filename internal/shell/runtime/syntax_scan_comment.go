package runtime

// commentStartsAt is commentStarts for a physical line that may go on from the one before it.
// After a backslash-newline, the # at the line's start follows the last character of that line,
// and belongs to its word when that character does: `${x\` then `#a}` is ${x#a}, and `echo
// 3:$\` then `{\`, `#\`, `3\` and `}` is ${#3}, as busybox and bash read them. The scan took
// the # for a comment, the } went with it, and the script was refused as missing one. After a
// line that ends in an operator, the logical line ends in a blank, and the # begins a comment
// there as before.
func (scanner *syntaxScanner) commentStartsAt(line string, index int) bool {
	if index > 0 || !scanner.joined || scanner.logical.Len() == 0 {
		return commentStarts(line, index)
	}
	logical := scanner.logical.String()
	return commentStarts(logical+"#", len(logical))
}
