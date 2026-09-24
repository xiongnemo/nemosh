package runtime

import (
	"context"
	"strings"
)

// casePattern is a case arm's pattern as the matcher reads it: expanded, unsplit, and with
// what was quoted made literal. A quoted `*` is a star and not a pattern: `case x in "*")`
// does not match x in busybox-w32 or bash 5.3, and neither does `"$p"` with p='*', or
// `a"*"` against ab. Here all three matched, because the expansion lost which characters
// had been quoted before the matcher saw them. An unquoted expansion stays a pattern, as
// in both: `case x in $p)` matches.
func (r Runtime) casePattern(ctx context.Context, candidate word, savedStatus int) string {
	expander := r.expandingAssignment()
	if !wordHasQuotedPart(candidate) {
		return strings.Join(expander.expandWord(ctx, candidate, savedStatus), "")
	}
	var pattern strings.Builder
	for _, part := range candidate.parts {
		text := strings.Join(expander.expandWord(ctx, word{parts: []wordPart{part}}, savedStatus), "")
		if part.quote != quoteUnquoted || part.kind == wordPartEscaped {
			text = literalIn(operandPattern, text)
		}
		pattern.WriteString(text)
	}
	return pattern.String()
}

func wordHasQuotedPart(item word) bool {
	for _, part := range item.parts {
		if part.quote != quoteUnquoted || part.kind == wordPartEscaped {
			return true
		}
	}
	return false
}
