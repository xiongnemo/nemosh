package runtime

import (
	"context"
	"fmt"
	"slices"
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
	if name, transform, ok := splitTransform(body); ok {
		fields, isList, err := r.transformList(ctx, name, transform)
		if err != nil {
			r.reportExpansionError(err)
			return nil, true
		}
		return fields, isList
	}
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
		return r.sliceList(ctx, elements, r.listIndices(name), word, name, savedStatus)
	case "/", "//", "/#", "/%":
		pattern := r.expandReplaceSpec(ctx, word, savedStatus)
		return mapList(elements, func(element string) string {
			return parameterReplace(element, operator, pattern, r.noCaseMatch())
		}), true
	case "^", "^^", ",", ",,":
		pattern := r.expandOperand(ctx, word, operandPattern, savedStatus)
		return mapList(elements, func(element string) string {
			return parameterCase(element, operator, pattern)
		}), true
	case "#", "##", "%", "%%":
		pattern := r.expandOperand(ctx, word, operandPattern, savedStatus)
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

// listIndices is the subscripts of the indexed array a list name stands for, in order, and
// nil for any other list: the positional parameters, an associative array, a scalar.
func (r Runtime) listIndices(name string) []int {
	reference, ok := parseArrayReference(name)
	if !ok || r.arrays.isAssociative(reference.name) {
		return nil
	}
	if _, isArray := r.arrays.get(reference.name); !isArray {
		return nil
	}
	return r.arrays.liveIndices(reference.name)
}

// sliceList is `${list:offset:length}`.
//
// The offset and the length are arithmetic, as they are for a string, and a negative
// offset counts from the end -- `${a[@]: -2}` is the last two, which needs the space
// for the same reason `${x: -2}` does.
//
// The rest is bash's, and was not here:
//
//   - An indexed array's offset is an index, not a position, so a sparse array is sliced
//     by its subscripts: `${a[@]:15:2}` over indices 33, 66 and 99 is the first two of
//     them. It counted fifteen elements into a list of three and gave nothing. A negative
//     offset counts back from one past the highest index, as a negative subscript does.
//   - An offset that reaches back past the start gives nothing. It was moved up to the
//     start, so `${a[*]: -5}` of four elements was all four.
//   - A negative length is an error, `substring expression < 0`. It is a count from the end
//     for a string, and it was one for a list too, so `${a[@]: 1: -3}` of five elements
//     gave the second where bash runs nothing.
//
// indices is the array's subscripts, in order, or nil for a list whose positions are its
// indices.
func (r Runtime) sliceList(ctx context.Context, elements []string, indices []int, spec, name string, savedStatus int) ([]string, bool) {
	sliced := r.sliceElements(ctx, elements, indices, spec, savedStatus)
	if strings.HasSuffix(name, "[*]") || name == "*" {
		// The `*` forms join into one field, as they do without an operator -- an empty
		// one when the slice is empty, which the caller takes as the value.
		return []string{strings.Join(sliced, r.starSeparator())}, true
	}
	return sliced, true
}

// sliceElements is the elements a slice selects, or none when it selects none or is wrong.
func (r Runtime) sliceElements(ctx context.Context, elements []string, indices []int, spec string, savedStatus int) []string {
	offsetText, lengthText, hasLength := splitSubstringSpec(spec)
	offset, err := r.substringNumber(ctx, offsetText, "offset", savedStatus)
	if err != nil {
		r.reportExpansionError(err)
		return nil
	}
	span := len(elements)
	if len(indices) > 0 {
		span = indices[len(indices)-1] + 1
	}
	if offset < 0 {
		offset += span
	}
	first := offset
	if indices != nil {
		first, _ = slices.BinarySearch(indices, offset)
	}
	if offset < 0 || first >= len(elements) {
		return nil
	}
	end := len(elements)
	if hasLength {
		length, err := r.substringNumber(ctx, lengthText, "length", savedStatus)
		if err == nil && length < 0 {
			err = fmt.Errorf("%s: substring expression < 0", strings.TrimSpace(lengthText))
		}
		if err != nil {
			r.reportExpansionError(err)
			return nil
		}
		end = min(first+length, end)
	}
	if end <= first {
		return nil
	}
	return append([]string(nil), elements[first:end]...)
}

func mapList(elements []string, apply func(string) string) []string {
	mapped := make([]string, 0, len(elements))
	for _, element := range elements {
		mapped = append(mapped, apply(element))
	}
	return mapped
}
