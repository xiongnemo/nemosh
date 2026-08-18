package runtime

import "sort"

// The bookkeeping that makes an indexed array sparse, split from array.go to stay under
// the 250-line ceiling. See the `present` field there for why it exists.

// mark records that an index is set.
func (a *shellArrays) mark(name string, indices ...int) {
	if a.present == nil {
		a.present = map[string]map[int]bool{}
	}
	if a.present[name] == nil {
		a.present[name] = map[int]bool{}
	}
	for _, index := range indices {
		a.present[name][index] = true
	}
}

// liveIndices is the set subscripts of a name, in order. This is what `${!a[@]}`
// answers and what the `[@]` and `[*]` forms are built from.
func (a *shellArrays) liveIndices(name string) []int {
	set := a.present[name]
	if set == nil {
		// A name written before this bookkeeping existed, or one whose slice was set
		// directly: every position counts, which is what dense meant.
		indices := make([]int, 0, len(a.values[name]))
		for index := range a.values[name] {
			indices = append(indices, index)
		}
		return indices
	}
	indices := make([]int, 0, len(set))
	for index := range set {
		indices = append(indices, index)
	}
	sort.Ints(indices)
	return indices
}

// liveValues is `${a[@]}`: the elements that are set, in index order, with no
// phantom fields for the gaps.
func (a *shellArrays) liveValues(name string) []string {
	elements := a.values[name]
	indices := a.liveIndices(name)
	values := make([]string, 0, len(indices))
	for _, index := range indices {
		if index < len(elements) {
			values = append(values, elements[index])
		}
	}
	return values
}
