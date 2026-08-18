package runtime

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// expandBracedParameter answers a `${...}` expansion. The operators are the ones
// POSIX 2.6.2 lists, plus the `${#name}` length form of 2.6.3.
//
// Only `-` and `:-` were implemented, and an unrecognised body fell through to
// the caller, which handed back the literal text: `${x%.txt}` expanded to the
// six characters `${x%.txt}` and exited 0. An operator that is not implemented
// has to say so rather than quietly become data, which is why the default case
// is an error now instead of a fallthrough.
func (r Runtime) expandBracedParameter(ctx context.Context, body string, savedStatus int) (string, error) {
	if body == "" {
		return "", fmt.Errorf("bad substitution: ${}")
	}
	if length, ok := strings.CutPrefix(body, "#"); ok && length != "" {
		return r.expandParameterLength(ctx, length, savedStatus)
	}
	// A body that is a parameter reference and nothing else: a name, a
	// positional number, or one of the special symbols. `${2}` and `${?}` are
	// the commonest, and reaching the operator split with them would find the
	// `?` and call it an operator with no name in front of it.
	// `${!name}` is indirection. Before the operator split, because the `!` is a
	// prefix rather than an operator between a name and a word. The array forms
	// `${!a[@]}` never reach here; see array.go.
	if indirect, ok := strings.CutPrefix(body, "!"); ok && indirect != "" {
		return r.expandIndirectParameter(ctx, indirect, savedStatus)
	}
	if isBareParameterReference(body) {
		value, set := r.lookupParameter(ctx, body, savedStatus)
		if !set {
			r.reportUnsetParameter(body)
		}
		return value, nil
	}
	name, operator, word, ok := splitParameterOperator(body)
	if !ok {
		return "", fmt.Errorf("bad substitution: ${%s}", body)
	}
	value, set := r.lookupParameter(ctx, name, savedStatus)
	switch operator {
	case "-", ":-", "=", ":=", "+", ":+", "?", ":?":
		return r.applyDefaultOperator(ctx, name, operator, word, value, set, savedStatus)
	case ":":
		return r.parameterSubstring(value, word)
	case "/", "//", "/#", "/%":
		return parameterReplace(value, operator, r.expandScalarParameterText(ctx, word, savedStatus)), nil
	case "^", "^^", ",", ",,":
		return parameterCase(value, operator, r.expandScalarParameterText(ctx, word, savedStatus)), nil
	default:
		return trimParameter(operator, value, r.expandScalarParameterText(ctx, word, savedStatus)), nil
	}
}

// The colon forms treat an empty value as absent; the bare forms only care
// whether the parameter is set at all.
func (r Runtime) applyDefaultOperator(ctx context.Context, name, operator, word, value string, set bool, savedStatus int) (string, error) {
	missing := !set
	if strings.HasPrefix(operator, ":") {
		missing = !set || value == ""
	}
	switch strings.TrimPrefix(operator, ":") {
	case "+":
		if missing {
			return "", nil
		}
		return r.expandScalarParameterText(ctx, word, savedStatus), nil
	case "?":
		if !missing {
			return value, nil
		}
		message := r.expandScalarParameterText(ctx, word, savedStatus)
		if message == "" {
			message = "parameter not set"
		}
		return "", fmt.Errorf("%s: %s", name, message)
	case "=":
		if !missing {
			return value, nil
		}
		// `=` assigns as well as substitutes, which is the only expansion that
		// changes the shell's state.
		assigned := r.expandScalarParameterText(ctx, word, savedStatus)
		if !isVariableName(name) {
			return "", fmt.Errorf("%s: cannot assign in this way", name)
		}
		r.vars[name] = assigned
		r.markVarMutation(name)
		return assigned, nil
	default:
		if missing {
			return r.expandScalarParameterText(ctx, word, savedStatus), nil
		}
		return value, nil
	}
}

// trimParameter is the #, ##, % and %% family: strip the shortest or longest
// matching prefix or suffix, where "matching" is the pattern language of 2.13.1
// rather than a literal comparison.
func trimParameter(operator, value, pattern string) string {
	switch operator {
	case "#":
		return trimPatternPrefix(value, pattern, false)
	case "##":
		return trimPatternPrefix(value, pattern, true)
	case "%":
		return trimPatternSuffix(value, pattern, false)
	default:
		return trimPatternSuffix(value, pattern, true)
	}
}

func trimPatternPrefix(value, pattern string, longest bool) string {
	best := -1
	for end := 0; end <= len(value); end++ {
		if !matchShellPattern(pattern, value[:end]) {
			continue
		}
		best = end
		if !longest {
			break
		}
	}
	if best < 0 {
		return value
	}
	return value[best:]
}

func trimPatternSuffix(value, pattern string, longest bool) string {
	best := -1
	for start := len(value); start >= 0; start-- {
		if !matchShellPattern(pattern, value[start:]) {
			continue
		}
		best = start
		if !longest {
			break
		}
	}
	if best < 0 {
		return value
	}
	return value[:best]
}

func (r Runtime) expandParameterLength(ctx context.Context, name string, savedStatus int) (string, error) {
	if !isVariableName(name) && name != "@" && name != "*" {
		return "", fmt.Errorf("bad substitution: ${#%s}", name)
	}
	if name == "@" || name == "*" {
		return strconv.Itoa(len(r.params.values)), nil
	}
	value, _ := r.lookupParameter(ctx, name, savedStatus)
	return strconv.Itoa(len([]rune(value))), nil
}

// splitParameterOperator finds the operator that separates the parameter name
// from the word after it. The two-character forms are tried first so `:-` is not
// read as a name ending in `:` followed by `-`.
func splitParameterOperator(body string) (string, string, string, bool) {
	for index := range len(body) {
		// Longest first at each position, and the `:x` defaults before a bare `:`,
		// which is what keeps `${x:-2}` a default and `${x: -2}` a substring. The
		// pairs `//`, `^^` and `,,` likewise precede their single forms.
		for _, operator := range [...]string{
			":-", ":=", ":+", ":?", "##", "%%", "//", "/#", "/%", "^^", ",,",
			":", "-", "=", "+", "?", "#", "%", "/", "^", ",",
		} {
			if !strings.HasPrefix(body[index:], operator) {
				continue
			}
			name := body[:index]
			if name == "" {
				return "", "", "", false
			}
			return name, operator, body[index+len(operator):], true
		}
	}
	return "", "", "", false
}

// lookupParameter reads a name the way an expansion sees it: a positional
// parameter by number, a special parameter by symbol, and anything else from
// the shell variables.
func (r Runtime) lookupParameter(ctx context.Context, name string, savedStatus int) (string, bool) {
	if number, err := strconv.Atoi(name); err == nil && number > 0 {
		if number <= len(r.params.values) {
			return r.params.values[number-1], true
		}
		return "", false
	}
	switch name {
	case "0", "?", "#", "@", "*", "-":
		return r.expandScalarParameterText(ctx, "$"+name, savedStatus), true
	}
	if value, set := r.vars[name]; set {
		return value, true
	}
	// After the stored variables, so a script that set one of these names sees what
	// it set. See special_vars.go.
	if value, ok := r.dynamicParameter(name); ok {
		return value, true
	}
	return "", false
}

func (r Runtime) expandScalarParameterText(ctx context.Context, text string, savedStatus int) string {
	if !strings.ContainsRune(text, '$') {
		return text
	}
	if !strings.HasPrefix(text, "$") || len(text) == 1 {
		// A reference somewhere other than the start: `${x:-pre${y}post}`. Returning
		// the text unchanged here is what left the inner reference sitting in the
		// output as its own characters.
		return r.expandEmbeddedParameters(ctx, text, savedStatus)
	}
	switch text {
	case "$0":
		return r.params.name
	case "$?":
		return strconv.Itoa(savedStatus)
	case "$#":
		return strconv.Itoa(len(r.params.values))
	case "$@", "$*":
		return strings.Join(r.params.values, " ")
	case "$-":
		return r.options.letters()
	}
	if len(text) == 2 && text[1] >= '1' && text[1] <= '9' {
		index := int(text[1] - '1')
		if index < len(r.params.values) {
			return r.params.values[index]
		}
		return ""
	}
	if isVariableName(strings.TrimPrefix(text, "$")) {
		return r.vars[strings.TrimPrefix(text, "$")]
	}
	// Anything else is text with references in it -- `${y}`, `pre$y post`, a nested
	// default. Looking the whole thing up as a variable name is what made
	// `${x:-${y}}` produce a stray brace. See expandEmbeddedParameters.
	return r.expandEmbeddedParameters(ctx, text, savedStatus)
}

func isBareParameterReference(body string) bool {
	if isVariableName(body) {
		return true
	}
	if number, err := strconv.Atoi(body); err == nil && number >= 0 {
		return true
	}
	switch body {
	case "?", "#", "@", "*", "-", "$", "!":
		return true
	}
	return false
}

func isVariableName(name string) bool {
	if name == "" || ('0' <= name[0] && name[0] <= '9') {
		return false
	}
	for i := 0; i < len(name); i++ {
		if !isNameByte(name[i]) {
			return false
		}
	}
	return true
}
