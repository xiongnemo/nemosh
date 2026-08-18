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
func (r Runtime) parameterSubstring(value, spec string) (string, error) {
	offsetText, lengthText, hasLength := splitSubstringSpec(spec)
	runes := []rune(value)
	offset, err := r.substringNumber(offsetText, "offset")
	if err != nil {
		return "", err
	}
	if offset < 0 {
		offset += len(runes)
	}
	offset = max(offset, 0)
	if offset > len(runes) {
		return "", nil
	}
	end := len(runes)
	if hasLength {
		length, err := r.substringNumber(lengthText, "length")
		if err != nil {
			return "", err
		}
		if length < 0 {
			// A negative length is an offset from the end rather than a count:
			// `${x:1:-1}` over abcdef is bcde, so it means "stop one short".
			end = len(runes) + length
		} else {
			end = offset + length
		}
	}
	end = min(end, len(runes))
	if end <= offset {
		return "", nil
	}
	return string(runes[offset:end]), nil
}

func (r Runtime) substringNumber(text, what string) (int, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0, nil
	}
	value, err := r.evaluateArithmetic(trimmed)
	if err != nil {
		return 0, fmt.Errorf("bad substitution: %s: %w", what, err)
	}
	return int(value), nil
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

// parameterReplace is `${name/pattern/replacement}` and its three variants: `//`
// for every match, `/#` anchored at the start, `/%` anchored at the end.
//
// The pattern is a shell pattern rather than a regular expression, and the match is
// greedy: measured, `${x/b*/-}` over `a1b2c` gives `a1-`, so `b*` took everything
// it could. A missing replacement deletes -- `${x//X}` is `${x//X/}`.
func parameterReplace(value, operator, spec string) string {
	pattern, replacement := splitReplacementSpec(spec)
	if pattern == "" {
		return value
	}
	switch operator {
	case "/#":
		if width, ok := longestPatternMatchAt(value, pattern, 0); ok {
			return replacement + value[width:]
		}
		return value
	case "/%":
		for start := 0; start <= len(value); start++ {
			if matchShellPattern(pattern, value[start:]) {
				return value[:start] + replacement
			}
		}
		return value
	}
	return replaceMatches(value, pattern, replacement, operator == "//")
}

func replaceMatches(value, pattern, replacement string, all bool) string {
	var out strings.Builder
	for index := 0; index <= len(value); {
		width, ok := longestPatternMatchAt(value, pattern, index)
		if !ok {
			if index == len(value) {
				break
			}
			out.WriteByte(value[index])
			index++
			continue
		}
		out.WriteString(replacement)
		if width == 0 {
			// An empty match would otherwise never advance. bash replaces once at
			// each position in that case rather than spinning.
			if index == len(value) {
				break
			}
			out.WriteByte(value[index])
			index++
		} else {
			index += width
		}
		if !all {
			out.WriteString(value[index:])
			return out.String()
		}
	}
	return out.String()
}

// longestPatternMatchAt reports how many bytes of value, starting at index, the
// pattern matches, taking the longest.
//
// Built on matchShellPattern rather than on a new matcher, because that one is
// already the shell's definition of a pattern and is already fuzzed. The cost is a
// scan per position, which is nothing at the sizes a variable holds.
func longestPatternMatchAt(value, pattern string, index int) (int, bool) {
	for end := len(value); end >= index; end-- {
		if matchShellPattern(pattern, value[index:end]) {
			return end - index, true
		}
	}
	return 0, false
}

// splitReplacementSpec cuts `pattern/replacement` at the first unescaped slash. A
// slash can be part of the pattern when escaped, which is what lets a path be
// matched.
func splitReplacementSpec(spec string) (string, string) {
	var pattern strings.Builder
	for index := 0; index < len(spec); index++ {
		if spec[index] == '\\' && index+1 < len(spec) && spec[index+1] == '/' {
			pattern.WriteByte('/')
			index++
			continue
		}
		if spec[index] == '/' {
			return pattern.String(), spec[index+1:]
		}
		pattern.WriteByte(spec[index])
	}
	return pattern.String(), ""
}

// parameterCase is `${name^^}`, `${name,,}`, `${name^}` and `${name,}`.
//
// The doubled forms convert every character and the single ones only the first. An
// optional pattern narrows which characters are touched: measured, `${x^^[ab]}`
// over `abc` gives `ABc`.
func parameterCase(value, operator, pattern string) string {
	if value == "" {
		return value
	}
	upper := operator == "^^" || operator == "^"
	all := operator == "^^" || operator == ",,"
	convert := unicode.ToLower
	if upper {
		convert = unicode.ToUpper
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

// expandIndirectParameter is `${!name}`: the value of the variable whose name this
// variable holds.
//
// The array forms `${!a[@]}` and `${!a[*]}` are subscripts rather than indirection
// and are answered before this is reached; see array.go.
func (r Runtime) expandIndirectParameter(ctx context.Context, name string, savedStatus int) (string, error) {
	// `${!prefix*}` and `${!prefix@}` are a different question sharing the `!`:
	// the *names* that begin with prefix, not the value one of them holds. Handled
	// here because otherwise it would ask for a variable called `HO*` and quietly
	// find nothing -- which is what it did for one commit, and worse than the
	// `bad substitution` it replaced.
	if prefix, ok := strings.CutSuffix(name, "*"); ok {
		return r.namesWithPrefix(prefix), nil
	}
	if prefix, ok := strings.CutSuffix(name, "@"); ok {
		// bash makes this one a field per name when quoted, where `*` is always
		// one word. Unquoted the two are identical because the result splits
		// anyway, and the quoted difference is not expressible on this path; the
		// joined form is the closer of the two.
		return r.namesWithPrefix(prefix), nil
	}
	target, set := r.lookupParameter(ctx, name, savedStatus)
	if !set || target == "" {
		// bash gives the empty string rather than an error, and a script testing
		// `${!ref}` for emptiness is asking a reasonable question.
		return "", nil
	}
	if !isVariableName(target) {
		return "", fmt.Errorf("%s: invalid variable name", target)
	}
	value, _ := r.lookupParameter(ctx, target, savedStatus)
	return value, nil
}

// namesWithPrefix is the set part of `${!prefix*}`: every set variable whose name
// begins with prefix, sorted so the answer is the same twice.
func (r Runtime) namesWithPrefix(prefix string) string {
	var names []string
	for name := range r.vars {
		if strings.HasPrefix(name, prefix) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return strings.Join(names, " ")
}
