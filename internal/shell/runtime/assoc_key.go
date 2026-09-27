package runtime

import (
	"context"
	"strings"
)

// resolveKey is an associative array's key: its subscript expanded as one word -- parameters,
// command substitutions, arithmetic, quote removal -- and neither split nor globbed, as bash
// expands it. Only `$name` and `${name}` were expanded, so `${m[$2]}` was the empty key and
// `m["$k"]` the literal text $k. Text with nothing to expand is the key as it stands, blanks
// and all.
func (r Runtime) resolveKey(ctx context.Context, subscript string) string {
	text := strings.TrimSpace(subscript)
	if !strings.ContainsAny(text, "$`'\"\\") {
		return text
	}
	tokens, err := scanShellTokens(text)
	if err != nil {
		return strings.Trim(text, `"'`)
	}
	expander := r.expandingAssignment()
	var fields []string
	for _, token := range tokens {
		if token.kind != tokenWord || token.parsed == nil {
			return strings.Trim(text, `"'`)
		}
		fields = append(fields, expander.expandWord(ctx, parseTypedWord(*token.parsed), 0)...)
	}
	return strings.Join(fields, " ")
}
