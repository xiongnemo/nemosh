package runtime

import "fmt"

// declareArrayKind makes name the kind of array `declare -a` or `-A` asks for, and answers
// false, having said why, where it cannot be. An array of one kind is not made the other, as
// in bash: `declare -A` over an indexed array emptied it. A scalar made an array is its
// element or key 0, as there; it was dropped.
func (r Runtime) declareArrayKind(name string, options declareOptions) bool {
	scalar, isScalar := r.vars[name]
	isScalar = isScalar && !r.isArrayName(name)
	switch {
	case options.associative && r.arrays.has(name):
		fmt.Fprintf(r.streams.Stderr, "declare: %s: cannot convert indexed to associative array\n", name)
		return false
	case options.indexed && r.arrays.isAssociative(name):
		fmt.Fprintf(r.streams.Stderr, "declare: %s: cannot convert associative to indexed array\n", name)
		return false
	case options.associative:
		r.arrays.declareAssociative(name)
		if isScalar {
			r.arrays.setKey(name, "0", scalar)
		}
	case options.indexed:
		if !r.arrays.has(name) {
			r.arrays.set(name, nil)
		}
		if isScalar {
			r.arrays.setElement(name, 0, scalar)
		}
	}
	return true
}
