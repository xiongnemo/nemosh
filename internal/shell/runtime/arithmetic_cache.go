package runtime

import "sync/atomic"

// An expression a loop evaluates is the same text every turn, `i+1` a hundred thousand times,
// and was split into tokens every turn, three allocations each. arithmeticTokensOf keeps the
// tokens of the last expression it saw in each of 64 slots, chosen by a hash of the text, and
// hands them out again for the same text: the parser only reads them. A slot is replaced
// whole, so two shells reading one at once see the old entry or the new, never half of either.

type arithmeticTokens struct {
	expression string
	tokens     []string
}

var arithmeticTokenCache [64]atomic.Pointer[arithmeticTokens]

func arithmeticTokensOf(expression string) []string {
	hash := uint32(2166136261)
	for index := 0; index < len(expression); index++ {
		hash = (hash ^ uint32(expression[index])) * 16777619
	}
	slot := &arithmeticTokenCache[hash%uint32(len(arithmeticTokenCache))]
	if entry := slot.Load(); entry != nil && entry.expression == expression {
		return entry.tokens
	}
	tokens := tokenizeArithmetic(expression)
	slot.Store(&arithmeticTokens{expression: expression, tokens: tokens})
	return tokens
}
