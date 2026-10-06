package runtime

import "strings"

// cutAssignment cuts `name=value`, `name[subscript]=value` and their `+=` at the assignment's
// own `=`: the first one past the subscript, which is arithmetic or a key and may hold an `=`
// of its own -- `a[x=1]=one`, `a[x==5]=t`, `A[k=v]=w`, `a[b[1]=2]=z` -- as bash reads them.
// Each was cut at the first `=`, so `a[x=1]=one` ran as a command of that name and `declare
// a[x=1]=one` answered `a[x: not a valid name`. busybox-w32 has no arrays to ask.
func cutAssignment(text string) (target, value string, found bool) {
	from := 0
	if open := strings.IndexByte(text, '['); open > 0 && isValidVariableName(text[:open]) {
		if end := subscriptClose(text, open); end > 0 {
			from = end + 1
		}
	}
	at := strings.IndexByte(text[from:], '=')
	if at < 0 {
		return text, "", false
	}
	return text[:from+at], text[from+at+1:], true
}

// assignedName is the variable an assignment's target names: its own, or an element's array.
func assignedName(target string) string {
	if reference, ok := parseArrayReference(target); ok {
		return reference.name
	}
	return target
}

// subscriptClose finds the `]` that closes the `[` at open, counting the brackets inside it and
// passing over what is quoted or escaped, so `m["a]b"]` closes at its last byte; -1 when none
// does.
func subscriptClose(text string, open int) int {
	depth := 0
	for index := open; index < len(text); index++ {
		switch text[index] {
		case '\\':
			index++
		case '\'':
			end := strings.IndexByte(text[index+1:], '\'')
			if end < 0 {
				return -1
			}
			index += end + 1
		case '"':
			for index++; index < len(text) && text[index] != '"'; index++ {
				if text[index] == '\\' {
					index++
				}
			}
		case '[':
			depth++
		case ']':
			if depth--; depth == 0 {
				return index
			}
		}
	}
	return -1
}
