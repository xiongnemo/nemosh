package runtime

import (
	"context"
	"fmt"
	"strings"
)

// Compound assignment with subscripts: `a=([2]=two [0]=zero)`, `declare -A m=([k]=v)`,
// `m+=([c]=3)`.
//
// **None of it worked, and all of it was quiet.** An element written `[k]=v` was a word like
// any other, so `a=([2]=two [0]=zero)` made a two-element array holding the text `[2]=two`
// and `[0]=zero`, and `declare -A m=([a]=1 [b]=2)` -- the way a lookup table is written in
// bash -- made an empty map. `${m[b]}` was empty with no error. And there were two copies of
// the list assignment, one for `a=(...)` and one for `declare a=(...)`, which is how neither
// learned it: this is the one now, and both call it.
//
// bash is the reference; busybox has no arrays.

// arrayElement is one element of a compound assignment: its value, and the subscript it was
// written with, if it had one.
type arrayElement struct {
	key   string
	keyed bool
	value string
}

// assignCompound replaces name with the elements of a compound assignment, or adds them to
// it for `+=`. A readonly name is refused as any assignment to one is -- this path had no
// check, so `readonly a; a=(1 2)` replaced it.
func (r Runtime) assignCompound(ctx context.Context, name, raw string, extend bool, savedStatus int) int {
	if r.isReadonly(name) {
		return r.refuseReadonly("", name)
	}
	elements := r.compoundElements(ctx, raw, savedStatus)
	if r.arrays.isAssociative(name) {
		r.assignAssociativeCompound(name, elements, extend)
		return 0
	}
	return r.assignIndexedCompound(name, elements, extend)
}

// assignIndexedCompound writes an indexed array. An element without a subscript goes at the
// index after the one before it, which is bash's rule: `a=([5]=x y)` puts y at 6.
func (r Runtime) assignIndexedCompound(name string, elements []arrayElement, extend bool) int {
	next := 0
	if extend {
		existing, _ := r.arrays.get(name)
		next = len(existing)
	} else {
		r.arrays.set(name, nil)
	}
	for _, element := range elements {
		index := next
		if element.keyed {
			value, err := r.evaluateArithmetic(element.key)
			if err != nil || value < 0 {
				fmt.Fprintf(r.streams.Stderr, "%s: [%s]: bad array subscript\n", name, element.key)
				return 1
			}
			index = int(value)
		}
		r.arrays.setElement(name, index, element.value)
		next = index + 1
	}
	r.syncArrayScalar(name)
	return 0
}

// assignAssociativeCompound writes an associative array. Words without subscripts are taken
// in pairs, key then value, which is what bash 5.1 made of `declare -A m=(k1 v1 k2 v2)`.
func (r Runtime) assignAssociativeCompound(name string, elements []arrayElement, extend bool) {
	if !extend {
		r.arrays.clearAssociative(name)
	}
	for index := 0; index < len(elements); index++ {
		element := elements[index]
		if element.keyed {
			r.arrays.setKey(name, element.key, element.value)
			continue
		}
		value := ""
		if index+1 < len(elements) && !elements[index+1].keyed {
			index++
			value = elements[index].value
		}
		r.arrays.setKey(name, element.value, value)
	}
}

// compoundElements lexes the text between the parentheses and expands each word, which is
// what keeps `"two words"` one element. Lexing rather than splitting on blanks: the quoting,
// the parameters and the command substitutions inside all have to work, and the lexer
// already knows how. A keyed element's value is expanded as an assignment is -- unsplit --
// and so is its subscript.
func (r Runtime) compoundElements(ctx context.Context, raw string, savedStatus int) []arrayElement {
	tokens, err := scanShellTokens(strings.TrimSpace(raw))
	if err != nil {
		return nil
	}
	var elements []arrayElement
	for _, token := range tokens {
		if token.kind != tokenWord || token.parsed == nil {
			continue
		}
		if key, value, keyed := splitKeyedElement(*token.parsed); keyed {
			elements = append(elements, arrayElement{
				key: r.expandUnsplit(ctx, key, savedStatus), keyed: true, value: r.expandUnsplit(ctx, value, savedStatus),
			})
			continue
		}
		for _, field := range r.expandCommandWord(ctx, *token.parsed, savedStatus) {
			elements = append(elements, arrayElement{value: field})
		}
	}
	return elements
}

func (r Runtime) expandUnsplit(ctx context.Context, item word, savedStatus int) string {
	return strings.Join(r.expandingAssignment().expandCommandWord(ctx, item, savedStatus), " ")
}

// splitKeyedElement cuts `[subscript]=value` at the `]=` that closes the subscript. The
// bracket has to open the word unquoted, and the `]=` has to be unquoted too: `"[a]=1"` is an
// element whose text happens to look like one. A `]` that is not followed by `=` means the
// word is not keyed at all -- `[abc]` is a pattern.
func splitKeyedElement(item word) (word, word, bool) {
	if len(item.parts) == 0 {
		return word{}, word{}, false
	}
	first := item.parts[0]
	if first.kind != wordPartLiteral || first.quote != quoteUnquoted || !strings.HasPrefix(first.text, "[") {
		return word{}, word{}, false
	}
	depth := 0
	var key []wordPart
	for index, part := range item.parts {
		if part.kind != wordPartLiteral || part.quote != quoteUnquoted {
			key = append(key, part)
			continue
		}
		start := 0
		if index == 0 {
			start = 1
		}
		for offset := start; offset < len(part.text); offset++ {
			switch part.text[offset] {
			case '[':
				depth++
			case ']':
				if depth > 0 {
					depth--
					continue
				}
				if offset+1 >= len(part.text) || part.text[offset+1] != '=' {
					return word{}, word{}, false
				}
				if offset > start {
					key = append(key, wordPart{kind: wordPartLiteral, text: part.text[start:offset]})
				}
				var value []wordPart
				if rest := part.text[offset+2:]; rest != "" {
					value = append(value, wordPart{kind: wordPartLiteral, text: rest})
				}
				value = append(value, item.parts[index+1:]...)
				return word{parts: key}, word{parts: value}, true
			}
		}
		if start < len(part.text) {
			key = append(key, wordPart{kind: wordPartLiteral, text: part.text[start:]})
		}
	}
	return word{}, word{}, false
}
