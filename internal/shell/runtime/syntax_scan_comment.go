package runtime

import "strings"

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

// commentStarts is whether the `#` at index begins a word, and so a comment: at a line's start,
// after a blank, and after `;`, `&`, `|` or `(`, as in `echo a;# c`, which both references read
// as a comment and nemosh ran as a command called `#`. A newline before it counts for the passes
// that read a whole script at once. After the `(` of an extended pattern it is the pattern's,
// as in bash's `[[ "#a" == @(#*) ]]`.
func commentStarts(line string, index int) bool {
	if index == 0 {
		return true
	}
	switch line[index-1] {
	case ' ', '\t', '\n', ';', '&', '|':
		return true
	case '(':
		return index < 2 || strings.IndexByte("@!?*+", line[index-2]) < 0
	}
	return false
}
