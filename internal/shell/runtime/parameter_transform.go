package runtime

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// The parameter expansions that are not in POSIX and are reached for constantly:
// substring, pattern replacement, case conversion, and indirect reference. Until
// now each of these was `bad substitution`, so a script written against bash --
// which is most scripts -- stopped at its first `${path%/*}`'s more useful cousin.
//
// Every expectation is measured against bash on the machine this was written on;
// the cases are in parameter_transform_test.go.
//
// Characters rather than bytes throughout. `${x:1:3}` counts characters in bash,
// so a value holding anything outside ASCII would otherwise be cut mid-rune -- the
// same defect the suggestion engine already had once.

// parameterSubstring is `${name:offset}` and `${name:offset:length}`.
//
// The two numbers are arithmetic expressions, not literals: `${x:n:2}` with n=2
// works in bash, so they go through the same evaluator `$(( ))` uses.
//
// A negative offset counts from the end, which is why `${x: -2}` needs the space --
// without it `${x:-2}` is the entirely different `:-` default operator. The
// operator splitter tries `:-` first, so both spellings already land where they
// should.
func (r Runtime) parameterSubstring(ctx context.Context, value, spec string, savedStatus int) (string, error) {
	offsetText, lengthText, hasLength := splitSubstringSpec(spec)
	runes := []rune(value)
	count := int64(len(runes))
	offset, err := r.substringNumber(ctx, offsetText, "offset", savedStatus)
	if err != nil {
		return "", err
	}
	if offset < 0 {
		offset += count
	}
	// Before the start is nothing, as both references read it: `${1: -8}` of seven characters
	// is empty. It was the whole value.
	if offset < 0 || offset > count {
		return "", nil
	}
	end := count
	if hasLength {
		length, err := r.substringNumber(ctx, lengthText, "length", savedStatus)
		if err != nil {
			return "", err
		}
		switch {
		case length < 0:
			// A negative length is an offset from the end rather than a count:
			// `${x:1:-1}` over abcdef is bcde, so it means "stop one short". One that ends
			// before the offset is bash's error, which abandons the line; it was nothing.
			end = count + length
			if end < offset {
				r.skipLineFor(fmt.Errorf("%s: substring expression < 0", lengthText))
				return "", nil
			}
		case length < count-offset:
			// Compared before it is added, so a length near the largest an int64 holds is
			// the rest of the value, as bash has it, and not an addition that wraps.
			end = offset + length
		}
	}
	if end <= offset {
		return "", nil
	}
	return string(runes[offset:end]), nil
}

// substringNumber evaluates an offset or a length, expanded first, as $(( )) expands what
// is in it: evaluated as written, `${s:$i:2}` stopped at the `$` as a syntax error. In 64
// bits on every build, as bash's intmax_t: an int is 32 on windows/386.
func (r Runtime) substringNumber(ctx context.Context, text, what string, savedStatus int) (int64, error) {
	trimmed := strings.TrimSpace(r.expandArithmeticText(ctx, text, savedStatus))
	if trimmed == "" {
		return 0, nil
	}
	value, err := r.evaluateArithmetic(trimmed)
	if err != nil {
		return 0, fmt.Errorf("bad substitution: %s: %w", what, err)
	}
	return value, nil
}

// splitSubstringSpec cuts `offset:length` at the top level, so a parenthesised
// expression keeps its own colons.
func splitSubstringSpec(spec string) (string, string, bool) {
	depth := 0
	for index := 0; index < len(spec); index++ {
		switch spec[index] {
		case '(':
			depth++
		case ')':
			depth--
		case ':':
			if depth == 0 {
				return spec[:index], spec[index+1:], true
			}
		}
	}
	return spec, "", false
}

// parameterCase is `${name^^}`, `${name,,}`, `${name~~}` and their single forms: raised,
// lowered, or toggled.
//
// The doubled forms convert every character and the single ones only the first. An
// optional pattern narrows which characters are touched: measured, `${x^^[ab]}`
// over `abc` gives `ABc`.
func parameterCase(value, operator, pattern string) string {
	if value == "" {
		return value
	}
	all := len(operator) == 2
	convert := toggleCase
	switch operator[0] {
	case '^':
		convert = unicode.ToUpper
	case ',':
		convert = unicode.ToLower
	}
	runes := []rune(value)
	for index, char := range runes {
		if !all && index > 0 {
			break
		}
		if pattern != "" && !matchShellPattern(pattern, string(char)) {
			continue
		}
		runes[index] = convert(char)
		if !all {
			break
		}
	}
	return string(runes)
}

// toggleCase is `~`'s conversion: an upper-case letter lowered, and any other raised, which
// leaves a character with no case as it was.
func toggleCase(char rune) rune {
	if unicode.IsUpper(char) {
		return unicode.ToLower(char)
	}
	return unicode.ToUpper(char)
}

// expandIndirectParameter is `${!ref...}` where one string is wanted: a here-document, an
// arithmetic expression, a condition's operand. See parameter_indirect.go; a whole array
// that ref names is joined with blanks, as `${a[@]}` is in those places.
//
// The array forms `${!a[@]}` and `${!a[*]}` are subscripts rather than indirection, and
// `${!prefix*}` and `${!prefix@}` the names with a prefix; both are answered before this is
// reached, see array.go and namesWithPrefix.
func (r Runtime) expandIndirectParameter(ctx context.Context, name string, savedStatus int) (string, error) {
	target, tail, err := r.indirectTarget(ctx, name, savedStatus)
	if err != nil || target == "" {
		return "", err
	}
	values := r.expandParameterPart(ctx, wordPart{kind: wordPartParameter, text: "${" + target + tail + "}"}, savedStatus)
	return strings.Join(values, " "), nil
}

// namesWithPrefix is `${!prefix@}` and `${!prefix*}`: every set variable or array whose
// name begins with prefix, sorted so the answer is the same twice. `@` is a field per name
// and `*` one field joined as `$*` is, bash's forms; busybox calls both a bad substitution.
// They were one field joined with a blank, so `"${!ab@}"` was one word and `${!ab*}` under
// `IFS=,` still had blanks in it.
func (r Runtime) namesWithPrefix(text string) ([]string, bool) {
	prefix, star := strings.CutSuffix(text, "*")
	if !star {
		var at bool
		if prefix, at = strings.CutSuffix(text, "@"); !at {
			return nil, false
		}
	}
	if !isVariableName(prefix) {
		return nil, false
	}
	seen := map[string]bool{}
	for name := range r.vars {
		seen[name] = true
	}
	for _, name := range r.arrays.indexedNames() {
		seen[name] = true
	}
	for _, name := range r.arrays.associativeNames() {
		seen[name] = true
	}
	var names []string
	for name := range seen {
		if strings.HasPrefix(name, prefix) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if star {
		return []string{strings.Join(names, r.starSeparator())}, true
	}
	return names, true
}
