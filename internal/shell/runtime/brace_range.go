package runtime

import (
	"strconv"
	"strings"
)

// The `..` forms of brace expansion, split from brace.go for the size ceiling.
//
//	$ echo {1..5}      1 2 3 4 5
//	$ echo {5..1}      5 4 3 2 1
//	$ echo {a..e}      a b c d e
//	$ echo {01..03}    01 02 03
//	$ echo {1..10..3}  1 4 7 10
//
// A range is only a range if every piece of it is plain unquoted text: `{1..$n}`
// is not, because the end is a parameter and brace expansion happens before
// parameters exist. bash leaves that one alone too.

// braceRange expands `FROM..TO[..STEP]`, reporting false when the contents are
// not a range.
func braceRange(inner []braceAtom) ([][]braceAtom, bool) {
	text, ok := plainAtomText(inner)
	if !ok {
		return nil, false
	}
	pieces := strings.Split(text, "..")
	if len(pieces) < 2 || len(pieces) > 3 {
		return nil, false
	}
	step := 1
	if len(pieces) == 3 {
		parsed, err := strconv.Atoi(pieces[2])
		if err != nil || parsed == 0 {
			return nil, false
		}
		// bash ignores the sign of the step and takes the direction from the
		// endpoints, so `{5..1..1}` and `{5..1..-1}` are the same.
		step = parsed
		if step < 0 {
			step = -step
		}
	}
	if values, ok := numericRange(pieces[0], pieces[1], step); ok {
		return atomiseRange(values), true
	}
	if values, ok := letterRange(pieces[0], pieces[1], step); ok {
		return atomiseRange(values), true
	}
	return nil, false
}

// plainAtomText returns the group's contents as text, and false if any atom is
// opaque -- a parameter, a substitution, anything quoted.
func plainAtomText(inner []braceAtom) (string, bool) {
	var text strings.Builder
	for _, atom := range inner {
		if atom.opaque != nil {
			return "", false
		}
		text.WriteRune(atom.literal)
	}
	return text.String(), true
}

// numericRange counts between two integers.
//
// The zero padding is bash's rule and the reason `{01..03}` is useful for
// filenames: if either endpoint is written with a leading zero, every value is
// padded to the width of the widest endpoint.
func numericRange(from, to string, step int) ([]string, bool) {
	start, err := strconv.Atoi(from)
	if err != nil {
		return nil, false
	}
	end, err := strconv.Atoi(to)
	if err != nil {
		return nil, false
	}
	width := 0
	if padded(from) || padded(to) {
		width = max(len(from), len(to))
	}
	var values []string
	if start <= end {
		for value := start; value <= end; value += step {
			values = append(values, padNumber(value, width))
		}
		return values, true
	}
	for value := start; value >= end; value -= step {
		values = append(values, padNumber(value, width))
	}
	return values, true
}

func padded(text string) bool {
	trimmed := strings.TrimPrefix(strings.TrimPrefix(text, "-"), "+")
	return len(trimmed) > 1 && trimmed[0] == '0'
}

func padNumber(value, width int) string {
	text := strconv.Itoa(value)
	if width == 0 || len(text) >= width {
		return text
	}
	negative := strings.HasPrefix(text, "-")
	digits := strings.TrimPrefix(text, "-")
	padding := strings.Repeat("0", width-len(text))
	if negative {
		return "-" + padding + digits
	}
	return padding + digits
}

// letterRange counts between two single letters, which is what `{a..e}` is.
//
// Both endpoints must be letters, and single ones. Measured: bash prints
// `{1..x}` and `{a..3}` unchanged, because a range is either numeric or
// alphabetic and never a mixture -- accepting one made `{1..x}` count from the
// digit `1` through the punctuation to `x`, which is nobody's intention. And
// `{aa..ac}` is not a range in bash either.
//
// `{A..e}` *does* expand, punctuation and all, because both ends are letters and
// the count is over code points. bash agrees, brackets and backtick included.
func letterRange(from, to string, step int) ([]string, bool) {
	fromRunes := []rune(from)
	toRunes := []rune(to)
	if len(fromRunes) != 1 || len(toRunes) != 1 {
		return nil, false
	}
	if !isASCIILetter(fromRunes[0]) || !isASCIILetter(toRunes[0]) {
		return nil, false
	}
	start, end := fromRunes[0], toRunes[0]
	var values []string
	if start <= end {
		for value := start; value <= end; value += rune(step) {
			values = append(values, string(value))
		}
		return values, true
	}
	for value := start; value >= end; value -= rune(step) {
		values = append(values, string(value))
	}
	return values, true
}

func atomiseRange(values []string) [][]braceAtom {
	alternatives := make([][]braceAtom, 0, len(values))
	for _, value := range values {
		atoms := make([]braceAtom, 0, len(value))
		for _, r := range value {
			atoms = append(atoms, braceAtom{literal: r})
		}
		alternatives = append(alternatives, atoms)
	}
	return alternatives
}

func isASCIILetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}
