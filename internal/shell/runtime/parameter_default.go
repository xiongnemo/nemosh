package runtime

import (
	"context"
	"strings"
)

type defaultParameterExpansion struct {
	name               string
	word               string
	useDefaultForEmpty bool
}

func (r Runtime) expandDefaultParameter(ctx context.Context, body string) (string, bool) {
	expansion, ok := parseDefaultParameterExpansion(body)
	if !ok {
		return "", false
	}
	value, set := r.vars[expansion.name]
	if !set || expansion.useDefaultForEmpty && value == "" {
		return r.expandArg(ctx, expansion.word), true
	}
	return value, true
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
