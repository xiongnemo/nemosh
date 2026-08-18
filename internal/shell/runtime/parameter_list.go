package runtime

import (
	"context"
	"strings"
)

// An operator applied to a *list* rather than to a string: `${@:2:2}`,
// `${a[@]:1:2}`, `${a[@]/x/y}`, `${a[@]^^}`.
//
// These all did the wrong thing quietly. `${@:2:2}` joined the positional parameters
// into one string and took a substring of *that*, so `set -- a b c d` gave `b ` rather
// than `b c`. The array forms found no variable called `a[@]` and gave the empty
// string. Slicing a list is the usual way to drop the first argument or take a window
// of one, so both are answers a script would act on.
//
// Every case is measured against bash, including the two that look like details and
// are not: `${@:0}` includes `$0`, so the offset indexes a list whose first entry is
// the shell's name; and an element containing a blank survives `"${a[@]:1:2}"` as one
// word, which is the whole reason arrays exist.

// expandListOperator answers an operator applied to a list, and reports whether the
// body was one.
//
// Tried before the scalar path, because the two disagree about what `${a[@]:1}` means
// and only this one is right. A name that is not a list falls through untouched.
func (r Runtime) expandListOperator(ctx context.Context, body string, savedStatus int) ([]string, bool) {
	name, operator, word, ok := splitParameterOperator(body)
	if !ok {
		return nil, false
	}
	elements, isList := r.parameterList(ctx, name)
	if !isList {
		return nil, false
	}
	switch operator {
	case ":":
		// Only the slice counts `$0`: measured, `${@:0}` is the shell's name followed
		// by the arguments, while `${@/x/y}` maps over the arguments alone. Putting
		// `$0` in the list for every operator made `${@/x/y}` answer `nemosh ay by`.
		if name == "@" || name == "*" {
			elements = append([]string{r.params.name}, elements...)
		}
		return r.sliceList(elements, word, name)
	case "/", "//", "/#", "/%":
		pattern := r.expandScalarParameterText(ctx, word, savedStatus)
		return mapList(elements, func(element string) string {
			return parameterReplace(element, operator, pattern)
		}), true
	case "^", "^^", ",", ",,":
		pattern := r.expandScalarParameterText(ctx, word, savedStatus)
		return mapList(elements, func(element string) string {
			return parameterCase(element, operator, pattern)
		}), true
	case "#", "##", "%", "%%":
		pattern := r.expandScalarParameterText(ctx, word, savedStatus)
		return mapList(elements, func(element string) string {
			return trimParameter(operator, element, pattern)
		}), true
	}
	// A default or an assignment operator on a whole list is not something bash does
	// either, so it is left to the scalar path to answer as it always has.
	return nil, false
}

// parameterList answers the elements a list name stands for, and reports whether the
// name is a list at all.
//
// `@` and `*` are the positional parameters, `$1` onwards. The slice operator adds
// `$0` in front of them itself, because it is the only one that counts it -- see the
// `:` case above.
func (r Runtime) parameterList(ctx context.Context, name string) ([]string, bool) {
	if name == "@" || name == "*" {
		return append([]string(nil), r.params.values...), true
	}
	reference, ok := parseArrayReference(name)
	if !ok || (reference.subscript != "@" && reference.subscript != "*") {
		return nil, false
	}
	elements, exists := r.elementsFor(ctx, reference)
	return elements, exists
}

// sliceList is `${list:offset:length}`.
//
// The offset and the length are arithmetic, as they are for a string, and a negative
// offset counts from the end -- `${a[@]: -2}` is the last two, which needs the space
// for the same reason `${x: -2}` does.
func (r Runtime) sliceList(elements []string, spec, name string) ([]string, bool) {
	offsetText, lengthText, hasLength := splitSubstringSpec(spec)
	offset, err := r.substringNumber(offsetText, "offset")
	if err != nil {
		r.reportExpansionError(err)
		return nil, true
	}
	if offset < 0 {
		offset += len(elements)
	}
	offset = max(offset, 0)
	if offset >= len(elements) {
		return nil, true
	}
	end := len(elements)
	if hasLength {
		length, err := r.substringNumber(lengthText, "length")
		if err != nil {
			r.reportExpansionError(err)
			return nil, true
		}
		if length < 0 {
			end = len(elements) + length
		} else {
			end = offset + length
		}
	}
	end = min(end, len(elements))
	if end <= offset {
		return nil, true
	}
	sliced := append([]string(nil), elements[offset:end]...)
	if strings.HasSuffix(name, "[*]") || name == "*" {
		// The `*` forms join into one field, as they do without an operator.
		return []string{strings.Join(sliced, " ")}, true
	}
	return sliced, true
}

func mapList(elements []string, apply func(string) string) []string {
	mapped := make([]string, 0, len(elements))
	for _, element := range elements {
		mapped = append(mapped, apply(element))
	}
	return mapped
}
