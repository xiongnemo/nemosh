package runtime

import "strings"

import "strconv"

type defaultParameterExpansion struct {
	name               string
	word               string
	useDefaultForEmpty bool
}

func (r Runtime) expandDefaultParameter(body string, savedStatus int) (string, bool) {
	expansion, ok := parseDefaultParameterExpansion(body)
	if !ok {
		return "", false
	}
	value, set := r.vars[expansion.name]
	if !set || expansion.useDefaultForEmpty && value == "" {
		return r.expandScalarParameterText(expansion.word, savedStatus), true
	}
	return value, true
}

func (r Runtime) expandScalarParameterText(text string, savedStatus int) string {
	if !strings.HasPrefix(text, "$") || len(text) == 1 {
		return text
	}
	switch text {
	case "$?":
		return strconv.Itoa(savedStatus)
	case "$#":
		return strconv.Itoa(len(r.params.values))
	case "$@", "$*":
		return strings.Join(r.params.values, " ")
	}
	if len(text) == 2 && text[1] >= '1' && text[1] <= '9' {
		index := int(text[1] - '1')
		if index < len(r.params.values) {
			return r.params.values[index]
		}
		return ""
	}
	return r.vars[strings.TrimPrefix(text, "$")]
}

func parseDefaultParameterExpansion(body string) (defaultParameterExpansion, bool) {
	if name, word, ok := strings.Cut(body, ":-"); ok && isVariableName(name) {
		return defaultParameterExpansion{name: name, word: word, useDefaultForEmpty: true}, true
	}
	if name, word, ok := strings.Cut(body, "-"); ok && isVariableName(name) {
		return defaultParameterExpansion{name: name, word: word}, true
	}
	return defaultParameterExpansion{}, false
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
