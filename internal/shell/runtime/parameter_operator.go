package runtime

import "strings"

// Where the parts of a `${...}` body are: the parameter, the operator after it, and the word
// after that. Split from parameter_default.go to keep it under the 250-line ceiling.

// splitParameterOperator finds the operator that separates the parameter name
// from the word after it. The two-character forms are tried first so `:-` is not
// read as a name ending in `:` followed by `-`.
func splitParameterOperator(body string) (string, string, string, bool) {
	// A special parameter is its one character, which may be an operator's too: `${?:-0}`,
	// `${-#*i}`, and `$#` itself in `${#-x}`. Read as the operator with no name before it,
	// each was a bad substitution, where both references take it.
	if len(body) > 1 && strings.IndexByte("#?-$@*", body[0]) >= 0 {
		if operator := parameterOperatorAt(body, 1); operator != "" {
			return body[:1], operator, body[1+len(operator):], true
		}
		return "", "", "", false
	}
	depth := 0
	for index := range len(body) {
		// Nothing inside an element's subscript is the operator: `${a[i+1]:-d}` has
		// `:-`, not `+`.
		switch body[index] {
		case '[':
			depth++
		case ']':
			if depth > 0 {
				depth--
				continue
			}
		}
		if depth > 0 {
			continue
		}
		if operator := parameterOperatorAt(body, index); operator != "" {
			name := body[:index]
			if name == "" {
				return "", "", "", false
			}
			return name, operator, body[index+len(operator):], true
		}
	}
	return "", "", "", false
}

// parameterOperatorAt is the operator that starts at index, or "". Longest first, and the
// `:x` defaults before a bare `:`, which is what keeps `${x:-2}` a default and `${x: -2}` a
// substring. The pairs `//`, `^^`, `,,` and `~~` likewise precede their single forms.
func parameterOperatorAt(body string, index int) string {
	for _, operator := range [...]string{
		":-", ":=", ":+", ":?", "##", "%%", "//", "/#", "/%", "^^", ",,", "~~",
		":", "-", "=", "+", "?", "#", "%", "/", "^", ",", "~",
	} {
		if strings.HasPrefix(body[index:], operator) {
			return operator
		}
	}
	return ""
}

// lengthOperand is what `${#...}` takes the length of, as busybox's parsesub reads it: a name
// or a number when a letter, a digit or `_` follows the `#`, and a special parameter when only
// that one character does, so `${##}` is the length of `$#`. Anything else is `$#` itself with
// an operator after it: with 25 parameters `${###}` is 25 and `${##2}` is 5 in both references.
// Every one of them was taken for a length, and was a bad substitution.
func lengthOperand(body string) (string, bool) {
	rest, ok := strings.CutPrefix(body, "#")
	if !ok || rest == "" || !isNameByte(rest[0]) && len(rest) > 1 {
		return "", false
	}
	return rest, true
}
