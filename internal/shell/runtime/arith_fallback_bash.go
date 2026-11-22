package runtime

import "strings"

// doubleParenExpansionEnd finds the last byte of the `$((` at index: its arithmetic's closing
// pair, or in bash mode, when no such pair closes it, the `)` of the command substitution it
// then is. bash reads `$((which x || echo y)2>/dev/null)` as a command whose first word is a
// subshell, which gnunet-gtk's packaging wrote, and `$(( echo 1 ... ) )` the same; busybox
// calls both an unterminated arithmetic expansion, which nemosh and sh keep. The lexer already
// takes a `$((` that is not arithmetic for a substitution; only the scans that find where a
// group ends refused it.
func doubleParenExpansionEnd(line string, index int, bash bool) (int, bool) {
	if end, ok := arithmeticExpansionEnd(line, index+3); ok || !bash {
		return end, ok
	}
	return commandSubstitutionEnd(line, index+2)
}

// bashWordSubstitutionEnd is the last byte of the `$(...)` or `$((...))` at index, read as bash
// mode reads one, and false where none begins or none closes.
func bashWordSubstitutionEnd(line string, index int) (int, bool) {
	if !strings.HasPrefix(line[index:], "$(") {
		return 0, false
	}
	if strings.HasPrefix(line[index:], "$((") {
		return doubleParenExpansionEnd(line, index, true)
	}
	return commandSubstitutionEnd(line, index+2)
}
