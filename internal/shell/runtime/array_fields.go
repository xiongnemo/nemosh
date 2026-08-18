package runtime

import "strings"

// Which `${...}` forms produce several fields rather than one string. Split from
// array.go to stay under the 250-line ceiling when the list operators arrived.

// isArrayAtReference reports whether the text is one of the forms that produces
// *several* fields rather than a string: `${name[@]}` and `${!name[@]}`.
//
// `${#name[@]}` is deliberately not one of them -- it is a count, and a count is
// one word. Leaving `${!a[@]}` out of this list made it expand to `0` instead of
// `0 1 2`, because the caller took only the first value.
func isArrayAtReference(text string) bool {
	body, ok := strings.CutPrefix(text, "${")
	if !ok {
		return false
	}
	body, ok = strings.CutSuffix(body, "}")
	if !ok {
		return false
	}
	body = strings.TrimPrefix(body, "!")
	if reference, ok := parseArrayReference(body); ok {
		return reference.subscript == "@"
	}
	// A list with an operator on it produces fields too: `${@:2:2}` is two
	// parameters and `${a[@]/x/y}` is one per element. Without this the caller took
	// only the first value, so `${a[@]:1:2}` expanded to `q` where it should be
	// `q r` -- the same failure `${!a[@]}` had, one form further along.
	return isListOperatorReference(body)
}

// isListOperatorReference reports whether a body is `@op`, `name[@]op` -- a list with
// an operator applied to it. The `*` forms are excluded because they join into one
// word, which is exactly what makes them the `*` forms.
func isListOperatorReference(body string) bool {
	name, _, _, ok := splitParameterOperator(body)
	if !ok {
		return false
	}
	if name == "@" {
		return true
	}
	reference, ok := parseArrayReference(name)
	return ok && reference.subscript == "@"
}
