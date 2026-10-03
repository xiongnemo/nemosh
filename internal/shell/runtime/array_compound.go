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
// written with, if it had one. appending is `[k]+=v`, which adds the value to the element's as
// `+=` adds to a variable's; it was the text `[k]+=v`, an element of its own. text is the word
// as read, for an error to name, and pair marks a key and a value read from two words; see
// keyValueElements.
type arrayElement struct {
	key       string
	keyed     bool
	appending bool
	pair      bool
	value     string
	text      string
}

// assignCompound replaces name with the elements of a compound assignment, or adds them to
// it for `+=`. A readonly name is refused as any assignment to one is -- this path had no
// check, so `readonly a; a=(1 2)` replaced it.
func (r Runtime) assignCompound(ctx context.Context, name, raw string, extend bool, savedStatus int) int {
	// Through a nameref, the array it leads to; see nameref.go. One that leads nowhere yet
	// becomes the array itself, and says so, as bash's does.
	if _, set := r.vars[name]; !set && r.attributes[name].nameref {
		fmt.Fprintf(r.streams.Stderr, "%swarning: %s: removing nameref attribute\n", r.diagnosticPrefix(), name)
		r.applyDeclaredAttributes(name, declareOptions{removed: "n"})
	}
	name, err := r.namerefBase(name)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "%s%v\n", r.diagnosticPrefix(), err)
		return 1
	}
	if r.isReadonly(name) {
		return r.refuseReadonly("", name)
	}
	associative := r.arrays.isAssociative(name)
	elements := r.compoundElements(ctx, raw, associative, savedStatus)
	if associative {
		return r.assignAssociativeCompound(name, elements, extend)
	}
	return r.assignIndexedCompound(name, elements, extend)
}

// assignIndexedCompound writes an indexed array. An element without a subscript goes at the
// index after the one before it, which is bash's rule: `a=([5]=x y)` puts y at 6.
func (r Runtime) assignIndexedCompound(name string, elements []arrayElement, extend bool) int {
	next := 0
	if extend {
		// A scalar appended to becomes the array's element 0, empty or not, as in bash:
		// `s=abc; s+=(d e)` is abc, d and e. Its value was dropped.
		if value, set := r.vars[name]; set && !r.arrays.has(name) {
			r.arrays.setElement(name, 0, value)
		}
		// After the highest index set, as bash appends.
		next = r.arrays.span(name)
	} else {
		r.arrays.set(name, nil)
	}
	status := r.writeIndexedElements(name, elements, next)
	r.syncArrayScalar(name)
	return status
}

// writeIndexedElements writes the elements in order, the first unsubscripted one at next. A
// negative subscript counts back from the end of the array as it stands by then, as in bash:
// `a=([5]=x [-1]=y)` is y at 5, and `a+=([-1]=z)` replaces the last element. It was refused.
// `[i]+=v` appends to what is at i by then, so `a=([1]=x [1]+=y)` is xy.
func (r Runtime) writeIndexedElements(name string, elements []arrayElement, next int) int {
	for _, element := range elements {
		index := next
		if element.keyed {
			number, err := r.evaluateArithmetic(element.key)
			position, within := countFromEnd(int(number), r.arrays.span(name))
			if err != nil || !within {
				return r.refuseElement(element.text + ": bad array subscript")
			}
			index = position
		}
		value := element.value
		if element.appending {
			current, _ := r.arrays.valueAt(name, index)
			value = r.appendedTo(name, current, value)
		}
		value, err := r.applyAttributes(name, value)
		if err != nil {
			fmt.Fprintf(r.streams.Stderr, "%s%s: %v\n", r.diagnosticPrefix(), name, err)
			return 1
		}
		r.arrays.setElement(name, index, value)
		next = index + 1
	}
	return 0
}

// assignAssociativeCompound writes an associative array. Every element needs its key, as in
// bash: a word without one is refused and the command abandoned, `m: 2: must use subscript`,
// and so is an empty key; they were paired up with the words beside them. `[k]+=v` appends to
// what k held before the assignment, as bash's does -- it writes the new array apart from the
// old one and puts it in place at the end -- so `m=([k]=1 [k]+=2)` is 2, and `m+=([k]+=2)`
// appends to the array as it is.
func (r Runtime) assignAssociativeCompound(name string, elements []arrayElement, extend bool) int {
	// clearAssociative puts a new array in the old one's place, so this is the old one still.
	before := r.arrays.associative[name]
	if !extend {
		r.arrays.clearAssociative(name)
	}
	for _, element := range elements {
		switch {
		case !element.keyed:
			return r.refuseElement(fmt.Sprintf("%s: %s: must use subscript when assigning associative array", name, element.text))
		case element.key == "" && element.pair:
			// Said and passed over, as bash does with a pair.
			fmt.Fprintf(r.streams.Stderr, "%s%s: bad array subscript\n", r.diagnosticPrefix(), element.text)
			continue
		case element.key == "":
			return r.refuseElement(element.text + ": bad array subscript")
		}
		value := element.value
		if element.appending {
			from := r.arrays.associative[name]
			if !extend {
				from = before
			}
			current := ""
			if from != nil {
				current = from.entries[element.key]
			}
			value = r.appendedTo(name, current, value)
		}
		r.arrays.setKey(name, element.key, r.attributedOrAsWritten(name, value))
	}
	return 0
}

// refuseElement says why an element of a compound assignment cannot be written and abandons
// the command, status 1, the elements before it written. bash does so wherever the list was:
// `declare -a m=([-1]=x); echo $?` never gets to the echo, as `m=([-1]=x)` does not. A plain
// assignment's other failures are failAssignment's.
func (r Runtime) refuseElement(message string) int {
	fmt.Fprintf(r.streams.Stderr, "%s%v\n", r.diagnosticPrefix(), message)
	if !r.expansion.shellError {
		r.expansion.shellError, r.expansion.discard = true, true
	}
	return 1
}

// attributedOrAsWritten is a map value with the name's attributes applied, or as written if
// they cannot be: a map's assignment is not refused element by element, in bash either.
func (r Runtime) attributedOrAsWritten(name, value string) string {
	if attributed, err := r.applyAttributes(name, value); err == nil {
		return attributed
	}
	return value
}

// compoundElements lexes the text between the parentheses and expands each word, which is
// what keeps `"two words"` one element. Lexing rather than splitting on blanks: the quoting,
// the parameters and the command substitutions inside all have to work, and the lexer
// already knows how. A keyed element's value is expanded as an assignment is -- unsplit, with
// a tilde at its start or after a `:` expanded, as bash 5.3 has `[2]=~:~`, and neither
// brace-expanded nor globbed: `[3]=*.py` is the star and `[5]=-{a,b}-` those characters, where
// they were the matches and `-a- -b-`, joined -- and its subscript is expanded the same way,
// with a tilde at its start. For an associative array, a word without a subscript is the last
// one expanded, as in bash, where the list stops at it.
func (r Runtime) compoundElements(ctx context.Context, raw string, associative bool, savedStatus int) []arrayElement {
	tokens, err := scanShellTokens(strings.TrimSpace(raw))
	if err != nil {
		return nil
	}
	var words []shellToken
	for _, token := range tokens {
		if token.kind == tokenWord && token.parsed != nil {
			words = append(words, token)
		}
	}
	if associative && len(words) > 0 && !opensWithBracket(*words[0].parsed) {
		return r.keyValueElements(ctx, words, savedStatus)
	}
	var elements []arrayElement
	for _, token := range words {
		key, value, appending, keyed := splitKeyedElement(*token.parsed)
		switch {
		case keyed:
			key.expandTilde = startsWithTilde(key)
			element := arrayElement{key: r.expandUnsplit(ctx, key, savedStatus), keyed: true, appending: appending, text: token.raw}
			if associative && element.key == "" {
				return append(elements, element)
			}
			value.valueTilde = true
			element.value = r.expandUnsplit(ctx, value, savedStatus)
			elements = append(elements, element)
		case associative:
			return append(elements, arrayElement{text: token.raw})
		default:
			for _, field := range r.expandCommandWord(ctx, *token.parsed, savedStatus) {
				elements = append(elements, arrayElement{value: field})
			}
		}
	}
	return elements
}

func (r Runtime) expandUnsplit(ctx context.Context, item word, savedStatus int) string {
	return strings.Join(r.expandingAssignment().expandWord(ctx, item, savedStatus), " ")
}

// opensWithBracket reports a word that begins with an unquoted `[`, which is how bash tells a
// list of keyed elements from one of keys and values; see keyValueElements.
func opensWithBracket(item word) bool {
	if len(item.parts) == 0 {
		return false
	}
	first := item.parts[0]
	return first.kind == wordPartLiteral && first.quote == quoteUnquoted && strings.HasPrefix(first.text, "[")
}

// splitKeyedElement cuts `[subscript]=value` at the `]=` that closes the subscript, or the
// `]+=`, which it reports. The bracket has to open the word unquoted, and the `]=` has to be
// unquoted too: `"[a]=1"` is an element whose text happens to look like one. A `]` that is not
// followed by `=` or `+=` means the word is not keyed at all -- `[abc]` is a pattern.
func splitKeyedElement(item word) (word, word, bool, bool) {
	if !opensWithBracket(item) {
		return word{}, word{}, false, false
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
				rest, appending, ok := afterSubscript(part.text[offset+1:])
				if !ok {
					return word{}, word{}, false, false
				}
				if offset > start {
					key = append(key, wordPart{kind: wordPartLiteral, text: part.text[start:offset]})
				}
				var value []wordPart
				if rest != "" {
					value = append(value, wordPart{kind: wordPartLiteral, text: rest})
				}
				value = append(value, item.parts[index+1:]...)
				return word{parts: key}, word{parts: value}, appending, true
			}
		}
		if start < len(part.text) {
			key = append(key, wordPart{kind: wordPartLiteral, text: part.text[start:]})
		}
	}
	return word{}, word{}, false, false
}

// afterSubscript is what follows a subscript's `]` once its `=` or `+=` is taken off, and
// whether it was `+=`.
func afterSubscript(text string) (string, bool, bool) {
	switch {
	case strings.HasPrefix(text, "="):
		return text[1:], false, true
	case strings.HasPrefix(text, "+="):
		return text[2:], true, true
	}
	return "", false, false
}
