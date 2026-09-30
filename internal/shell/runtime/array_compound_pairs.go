package runtime

import "context"

// keyValueElements reads an associative array's list as keys and values in turn, which is
// what bash makes of one whose first word does not open with a `[`: `declare -A m=(k1 v1 k2
// v2)`. The whole list is read so, a word written `[k]=v` being that text, as there; it was a
// key and value of its own, and the words around it paired up without it. A key with no value
// has an empty one.
//
// Each word is expanded whole, as in bash, with a tilde at its start: `($list x)` is the key
// `a b` when list is `a b`, `(*.sh v)` the key `*.sh`, and `("$@")` one key. They were split,
// globbed and brace-expanded into as many words as they made.
func (r Runtime) keyValueElements(ctx context.Context, words []shellToken, savedStatus int) []arrayElement {
	var elements []arrayElement
	for index := 0; index < len(words); index += 2 {
		element := arrayElement{key: r.expandUnsplit(ctx, *words[index].parsed, savedStatus), keyed: true, pair: true, text: words[index].raw}
		if index+1 < len(words) {
			element.value = r.expandUnsplit(ctx, *words[index+1].parsed, savedStatus)
		}
		elements = append(elements, element)
	}
	return elements
}
