package runtime

import (
	"fmt"
	"strings"
)

// arrayLiteralEnd finds the `)` that closes the array literal whose `(` is at open, and
// refuses what bash refuses between the two.
//
// The elements are words, so an operator among them is a syntax error: `a=(1 & 2)`,
// `a=(x | y)`, `a=(p; q)`, and a literal written one element a line with a stray `&` on
// one of them. The operator was dropped without a word, so the literal meant something
// other than what was written and the script went on. It is refused as the script is
// parsed, as every other syntax error is here, so none of the script runs; bash, which
// reads a script a line at a time, runs what came before it.
//
// The elements are lexed again at run time, by compoundElements. Lexing them here as well
// is what lets a mistake among them stop the script before it starts. A budget of their
// own, because counting them twice against the script's would halve the size of the
// largest literal it can hold.
func arrayLiteralEnd(line string, open, depth int) (int, error) {
	end, ok := matchingParenthesis(line, open)
	if !ok {
		return 0, fmt.Errorf("%w: missing ) for array assignment", ErrIncompleteScript)
	}
	tokens, err := scanShellTokensWithBudget(line[open+1:end], &parseBudget{}, depth)
	if err != nil {
		return 0, err
	}
	for _, token := range tokens {
		if token.kind != tokenWord {
			return 0, fmt.Errorf("syntax error: unexpected %s in an array assignment", token.value)
		}
		if hasUnquotedSemicolon(*token.parsed) {
			return 0, fmt.Errorf("syntax error: unexpected ; in an array assignment")
		}
	}
	return end, nil
}

// hasUnquotedSemicolon reports whether a word holds a `;` that nothing quotes. The lexer
// leaves `;` in a word, because the line was cut at every separator before the lexer saw
// it; one inside an array literal survives that, being inside its parentheses.
func hasUnquotedSemicolon(item word) bool {
	for _, part := range item.parts {
		if part.kind == wordPartLiteral && part.quote == quoteUnquoted && strings.Contains(part.text, ";") {
			return true
		}
	}
	return false
}
