package runtime

import "unicode"

// POSIX character classes in a bracket expression: `[[:upper:]]`, `[[:digit:][:space:]]`,
// `[![:alnum:]_]`. The matcher knew ranges and single characters only, so the class's own `]`
// closed the bracket early and `case A in [[:upper:]])` matched nothing -- in case, in
// `[[ == ]]`, in `${x//[[:space:]]/}` and in globs alike. Both references match them.

// characterClassEnd reports the index just past a `[:name:]` beginning at start.
func characterClassEnd(pattern []rune, start int) (int, bool) {
	if start+1 >= len(pattern) || pattern[start] != '[' || pattern[start+1] != ':' {
		return 0, false
	}
	for index := start + 2; index+1 < len(pattern); index++ {
		if pattern[index] == ':' && pattern[index+1] == ']' {
			return index + 2, true
		}
		if pattern[index] == ']' {
			return 0, false
		}
	}
	return 0, false
}

// inCharacterClass is whether char belongs to the named class. An unknown name matches
// nothing, as in both references.
func inCharacterClass(name string, char rune) bool {
	switch name {
	case "alpha":
		return unicode.IsLetter(char)
	case "digit":
		return '0' <= char && char <= '9'
	case "alnum":
		return unicode.IsLetter(char) || '0' <= char && char <= '9'
	case "upper":
		return unicode.IsUpper(char)
	case "lower":
		return unicode.IsLower(char)
	case "space":
		return unicode.IsSpace(char)
	case "blank":
		return char == ' ' || char == '\t'
	case "punct":
		return unicode.IsPunct(char) || unicode.IsSymbol(char)
	case "print":
		return unicode.IsPrint(char)
	case "graph":
		return unicode.IsPrint(char) && char != ' '
	case "cntrl":
		return unicode.IsControl(char)
	case "xdigit":
		return '0' <= char && char <= '9' || 'a' <= char && char <= 'f' || 'A' <= char && char <= 'F'
	}
	return false
}
