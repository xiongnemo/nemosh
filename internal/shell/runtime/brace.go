package runtime

import "strings"

// Brace expansion: `{a,b}`, `pre{a,b}post`, `{1..5}`, `{a..e}`, `{01..03}`,
// `{1..10..3}`, and any nesting of those.
//
// Not POSIX -- dash has none of it -- so this is a deliberate extension, taken
// from bash because it is what fingers do. Measured against bash throughout:
//
//	$ echo {a,b}          a b
//	$ echo pre{a,b}post   preapost prebpost
//	$ echo {1..5}         1 2 3 4 5
//	$ echo {5..1}         5 4 3 2 1
//	$ echo {01..03}       01 02 03
//	$ echo {1..10..3}     1 4 7 10
//	$ echo {a,b}{1,2}     a1 a2 b1 b2
//	$ echo {a,b{c,d}}     a bc bd
//	$ echo {a}            {a}       -- no comma and no range: left alone
//	$ echo {}             {}
//	$ echo "{a,b}"        {a,b}     -- quoted, so not a brace group at all
//	$ echo \{a,b\}        {a,b}
//
// **It happens before every other expansion**, which is the fact that decides
// the implementation. Measured: with `x=1`, `echo {$x,2}` prints `1 2`, so the
// split cannot be done on expanded text -- the alternatives have to carry the
// unexpanded parameter with them.
//
// So this works on the word's *parts* rather than on a string. Each unquoted
// literal contributes its characters, and everything else -- a parameter, a
// command substitution, an escape, anything quoted -- becomes one opaque atom
// that no brace can be found inside. That is also what makes `"{a,b}"` and
// `\{a,b\}` come out literal without a special case: their braces are not in an
// unquoted literal, so they are never seen.

// braceAtom is one character of an unquoted literal, or one untouchable part.
type braceAtom struct {
	literal rune
	opaque  *wordPart
}

func (a braceAtom) is(r rune) bool { return a.opaque == nil && a.literal == r }

// expandBraceWord turns one word into the words its braces produce, in order.
// A word with no brace group comes back as itself.
func expandBraceWord(item word) []word {
	atoms := braceAtomsOf(item)
	expanded := expandBraceAtoms(atoms)
	if len(expanded) == 1 {
		// Nothing was expanded, so the original word is returned untouched --
		// including its quotedEmpty and expandTilde flags, which a rebuild would
		// have to reconstruct.
		return []word{item}
	}
	words := make([]word, 0, len(expanded))
	for _, sequence := range expanded {
		words = append(words, wordFromBraceAtoms(sequence, item))
	}
	return words
}

func braceAtomsOf(item word) []braceAtom {
	var atoms []braceAtom
	for index := range item.parts {
		part := item.parts[index]
		if part.kind != wordPartLiteral || part.quote != quoteUnquoted {
			atoms = append(atoms, braceAtom{opaque: &item.parts[index]})
			continue
		}
		for _, r := range part.text {
			atoms = append(atoms, braceAtom{literal: r})
		}
	}
	return atoms
}

// wordFromBraceAtoms rebuilds a word, merging runs of literal characters back
// into single parts so the result looks like something the lexer could have
// produced.
func wordFromBraceAtoms(atoms []braceAtom, original word) word {
	rebuilt := word{quotedEmpty: original.quotedEmpty}
	var literal strings.Builder
	flush := func() {
		if literal.Len() == 0 {
			return
		}
		rebuilt.parts = append(rebuilt.parts, wordPart{kind: wordPartLiteral, text: literal.String()})
		literal.Reset()
	}
	for _, atom := range atoms {
		if atom.opaque != nil {
			flush()
			rebuilt.parts = append(rebuilt.parts, *atom.opaque)
			continue
		}
		literal.WriteRune(atom.literal)
	}
	flush()
	// Recomputed rather than copied: `{~,/tmp}` produces one word that starts
	// with a tilde and one that does not, and carrying the original's answer
	// would be wrong for one of them.
	rebuilt.expandTilde = startsWithTilde(rebuilt)
	return rebuilt
}

func startsWithTilde(item word) bool {
	if len(item.parts) == 0 {
		return false
	}
	first := item.parts[0]
	if first.kind != wordPartLiteral || first.quote != quoteUnquoted {
		return false
	}
	return first.text == "~" || strings.HasPrefix(first.text, "~/")
}

// expandBraceAtoms is the expansion itself: find the first group, expand it, and
// recurse on what each alternative produced so nesting and several groups in one
// word both come out right.
func expandBraceAtoms(atoms []braceAtom) [][]braceAtom {
	open, close, ok := firstBraceGroup(atoms)
	if !ok {
		return [][]braceAtom{atoms}
	}
	inner := atoms[open+1 : close]
	alternatives, ok := braceAlternatives(inner)
	if !ok {
		// A group with neither a comma nor a range is not a group: bash prints
		// `{a}` and `{}` unchanged. Scanning continues after it rather than
		// stopping, so `{a}{b,c}` still expands the second.
		rest := expandBraceAtoms(atoms[close+1:])
		results := make([][]braceAtom, 0, len(rest))
		for _, tail := range rest {
			results = append(results, concatAtoms(atoms[:close+1], tail))
		}
		return results
	}
	var results [][]braceAtom
	for _, alternative := range alternatives {
		// The alternative is expanded too, which is what makes `{a,b{c,d}}`
		// work, and then the suffix, which is what makes `{a,b}{1,2}` a product
		// rather than a concatenation.
		for _, expandedAlternative := range expandBraceAtoms(alternative) {
			head := concatAtoms(atoms[:open], expandedAlternative)
			for _, tail := range expandBraceAtoms(atoms[close+1:]) {
				results = append(results, concatAtoms(head, tail))
			}
		}
	}
	return results
}

func concatAtoms(left, right []braceAtom) []braceAtom {
	joined := make([]braceAtom, 0, len(left)+len(right))
	joined = append(joined, left...)
	return append(joined, right...)
}

// firstBraceGroup finds the first `{` that has a matching `}`, counting nesting.
func firstBraceGroup(atoms []braceAtom) (int, int, bool) {
	for index := range atoms {
		if !atoms[index].is('{') {
			continue
		}
		depth := 0
		for end := index; end < len(atoms); end++ {
			switch {
			case atoms[end].is('{'):
				depth++
			case atoms[end].is('}'):
				depth--
				if depth == 0 {
					return index, end, true
				}
			}
		}
		// An unmatched `{` is literal, and so is everything after it: bash leaves
		// `echo {a,b` alone entirely.
		return 0, 0, false
	}
	return 0, 0, false
}

// braceAlternatives splits a group's contents on top-level commas, or expands a
// `..` range. It reports false when the contents are neither.
func braceAlternatives(inner []braceAtom) ([][]braceAtom, bool) {
	if commas := splitTopLevelCommas(inner); commas != nil {
		return commas, true
	}
	return braceRange(inner)
}

func splitTopLevelCommas(inner []braceAtom) [][]braceAtom {
	depth := 0
	var parts [][]braceAtom
	start := 0
	found := false
	for index, atom := range inner {
		switch {
		case atom.is('{'):
			depth++
		case atom.is('}'):
			depth--
		case atom.is(',') && depth == 0:
			parts = append(parts, inner[start:index])
			start = index + 1
			found = true
		}
	}
	if !found {
		return nil
	}
	// The trailing piece is included even when empty, which is why `{a,}`
	// produces two words -- `a` and nothing -- rather than one.
	return append(parts, inner[start:])
}
