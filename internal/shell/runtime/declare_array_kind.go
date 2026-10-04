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
		fmt.Fprintf(r.streams.Stderr, "%sdeclare: %s: cannot convert indexed to associative array\n", r.diagnosticPrefix(), name)
		return false
	case options.indexed && r.arrays.isAssociative(name):
		fmt.Fprintf(r.streams.Stderr, "%sdeclare: %s: cannot convert associative to indexed array\n", r.diagnosticPrefix(), name)
		return false
	case options.associative:
		r.arrays.declareUnassigned(name, true)
		if isScalar {
			r.arrays.setKey(name, "0", scalar)
		}
	case options.indexed:
		r.arrays.declareUnassigned(name, false)
		if isScalar {
			r.arrays.setElement(name, 0, scalar)
		}
	}
	return true
}

// declareUnassigned makes a name that is no array yet one of the kind asked for, declared and
// never assigned, as bash's `declare -a a` and `local -A m` leave it: unset, so declare -p
// writes it `declare -a a` where `a=()` is `declare -a a=()`, and under `set -u` `${#a[@]}`
// is an error. It was the second. Anything stored in it makes it assigned.
func (a *shellArrays) declareUnassigned(name string, associative bool) {
	switch {
	case associative && !a.isAssociative(name):
		a.declareAssociative(name)
		a.associative[name].unassigned = true
	case !associative && !a.has(name):
		array := newIndexedArray()
		array.unassigned = true
		a.put(name, array)
	}
}

// unassigned reports an array declared and never assigned.
func (a *shellArrays) unassigned(name string) bool {
	if a == nil {
		return false
	}
	if array, ok := a.associative[name]; ok {
		return array.unassigned
	}
	array, ok := a.indexed[name]
	return ok && array.unassigned
}
