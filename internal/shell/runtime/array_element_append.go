package runtime

import (
	"context"
	"strconv"
)

// subscriptOnce is an indexed element's reference with its subscript evaluated to the number it
// names, for a write that reads the element first -- `a[i++]+=x`, `((b[i++] += 10))`,
// `((b[i++]++))` -- so that what the subscript does happens once, as in bash. It happened for
// the read and again for the write, which stepped i twice and wrote the next element. An
// associative key, `@` and `*` are left as written. The error is an arithmetic one, and said.
func (r Runtime) subscriptOnce(ctx context.Context, reference arrayReference) (arrayReference, error) {
	base, err := r.namerefBase(reference.name)
	if err != nil || r.arrays.isAssociative(base) || reference.subscript == "@" || reference.subscript == "*" {
		return reference, nil
	}
	index, err := r.resolveSubscript(ctx, reference.subscript)
	if err != nil {
		return reference, err
	}
	reference.subscript = strconv.Itoa(index)
	return reference, nil
}

// appendedElement is what `name[subscript]+=value` writes, and the element to write it to; see
// subscriptOnce. false is a subscript that was an arithmetic error.
func (r Runtime) appendedElement(ctx context.Context, reference arrayReference, value string) (arrayReference, string, bool) {
	reference, err := r.subscriptOnce(ctx, reference)
	if err != nil {
		return reference, "", false
	}
	// An element before the front of the array has nothing to read, and the write refuses it.
	if base, err := r.namerefBase(reference.name); err == nil && !r.arrays.isAssociative(base) {
		if index, err := strconv.Atoi(reference.subscript); err == nil {
			if _, within := countFromEnd(index, r.arrays.span(base)); !within {
				return reference, value, true
			}
		}
	}
	return reference, r.appendedValue(reference.name+"["+reference.subscript+"]", value), true
}
