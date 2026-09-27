package runtime

import (
	"strconv"
	"strings"

	"github.com/xiongnemo/nemosh/internal/shellquote"
)

// `${a[@]@K}` and `${a[@]@k}`, bash's, over an array: its subscripts and values in pairs.
// K is one word that an array literal reads back, `0 "a" 1 "b c"`, each value quoted as
// declare -p quotes it and each key as declare -p writes a key. k is a word each, unquoted,
// which is what a loop over `k v` pairs wants. Anything that is not an array -- a scalar,
// one element, the positional parameters -- is quoted as @Q quotes it. Both were refused by
// name; busybox has neither.

// keyValueWords answers K or k over an indexed or associative array, and reports whether
// the name was one.
func (r Runtime) keyValueWords(reference arrayReference, operator byte) ([]string, bool) {
	keys, values, associative, ok := r.arrayPairs(reference.name)
	if !ok {
		return nil, false
	}
	star := reference.subscript == "*"
	if operator == 'k' {
		words := make([]string, 0, 2*len(keys))
		for index := range keys {
			words = append(words, keys[index], values[index])
		}
		if star {
			return []string{strings.Join(words, r.starSeparator())}, true
		}
		return words, true
	}
	if len(keys) == 0 {
		// As `"${a[@]}"` and `"${a[*]}"` of an empty array: no word, and one empty word.
		if star {
			return []string{""}, true
		}
		return nil, true
	}
	pairs := make([]string, 0, len(keys))
	for index, key := range keys {
		if associative {
			key = shellquote.Key(key)
		}
		pairs = append(pairs, key+" "+shellquote.Double(values[index]))
	}
	text := strings.Join(pairs, " ")
	if associative {
		// bash leaves a blank at the end of an associative array's, as its declare -p does.
		text += " "
	}
	return []string{text}, true
}

// arrayPairs is an array's subscripts, as text, and its values, in order, and whether it is
// associative. A name that is no array is not one.
func (r Runtime) arrayPairs(name string) ([]string, []string, bool, bool) {
	if r.arrays.isAssociative(name) {
		keys := r.arrays.keysOf(name)
		values := make([]string, len(keys))
		for index, key := range keys {
			values[index], _ = r.arrays.lookupKey(name, key)
		}
		return keys, values, true, true
	}
	elements, ok := r.arrays.get(name)
	if !ok {
		return nil, nil, false, false
	}
	indices := r.arrays.liveIndices(name)
	keys := make([]string, len(indices))
	values := make([]string, len(indices))
	for position, index := range indices {
		keys[position], values[position] = strconv.Itoa(index), elements[index]
	}
	return keys, values, false, true
}
