package runtime

import (
	"context"
	"strconv"
	"strings"
)

// expandArrayParameter answers the array forms of `${...}`, reporting whether the
// body was one.
//
//	${a[0]}    one element
//	${a[@]}    every element, one field each
//	${a[*]}    every element, joined -- so a single field the caller may split
//	${#a[@]}   how many
//	${#a[0]}   the length of that element
//	${!a[@]}   the subscripts
func (r Runtime) expandArrayParameter(ctx context.Context, body string) ([]string, bool) {
	if count, ok := strings.CutPrefix(body, "#"); ok {
		reference, ok := parseArrayReference(count)
		if !ok {
			return nil, false
		}
		elements, exists := r.elementsFor(ctx, reference)
		if reference.subscript == "@" || reference.subscript == "*" {
			return []string{strconv.Itoa(len(elements))}, true
		}
		if !exists || len(elements) == 0 {
			return []string{"0"}, true
		}
		return []string{strconv.Itoa(len([]rune(elements[0])))}, true
	}
	if indices, ok := strings.CutPrefix(body, "!"); ok {
		if names, ok := r.namesWithPrefix(indices); ok {
			return names, true
		}
		reference, ok := parseArrayReference(indices)
		if !ok || (reference.subscript != "@" && reference.subscript != "*") {
			return nil, false
		}
		keys := r.arrayIndices(reference.name)
		if reference.subscript == "*" {
			// One field, as `${a[*]}` is. The caller takes one value from a `*` form, so
			// returning each key gave only the first: `0` for `0 1 2`.
			return []string{strings.Join(keys, r.starSeparator())}, true
		}
		return keys, true
	}
	reference, ok := parseArrayReference(body)
	if !ok {
		return nil, false
	}
	elements, _ := r.elementsFor(ctx, reference)
	if reference.subscript == "*" {
		// Joined into one field. Unquoted, the caller splits it on IFS again --
		// which is bash's behaviour and the reason `"${a[*]}"` is the form that
		// yields a single word.
		return []string{strings.Join(elements, r.starSeparator())}, true
	}
	if len(elements) == 0 {
		// `${a[@]}` of an empty array is no field at all, as `$@` with no parameters is;
		// one element that is unset is still an empty string.
		if reference.subscript == "@" {
			return nil, true
		}
		return []string{""}, true
	}
	return elements, true
}
