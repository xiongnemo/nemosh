package runtime

import "context"

// variableIsSet is `[[ -v name ]]`: whether a name is set, empty or not. The form a script
// uses to tell "unset" from "empty" under `set -u` without tripping it, and bash has it;
// here `-v` was not an operator, so `[[ -v x ]]` read as a test of the string "-v" and was
// true whatever x was.
//
// A subscript names one element -- an index is arithmetic, a key expands -- and `[@]` any
// of them. A bare array name is its element zero, as bash has it: `a=([3]=z)` is not set
// by that test. Every case is measured against bash.
func (r Runtime) variableIsSet(ctx context.Context, name string) bool {
	if reference, ok := parseArrayReference(name); ok {
		elements, _ := r.elementsFor(ctx, reference)
		return len(elements) > 0
	}
	if _, isArray := r.arrays.get(name); isArray || r.arrays.isAssociative(name) {
		elements, _ := r.elementsFor(ctx, arrayReference{name: name, subscript: "0"})
		return len(elements) > 0
	}
	_, set := r.lookupParameter(ctx, name, 0)
	return set
}
