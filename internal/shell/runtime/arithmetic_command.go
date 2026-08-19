package runtime

// `((expr))` -- the arithmetic command.
//
// It parsed as a subshell containing a subshell, so `((i++))` ran a command named
// `i++` and reported it not found. That is how `for ((i=0;i<3;i++))` failed too,
// and between them they are how most loops with a counter are written.
//
// bash documents `((expr))` as equivalent to `let "expr"`, and this shell already
// has `let`, so that is exactly what it becomes: the lexer emits two words, `let`
// and the expression, and everything downstream -- the status, the assignment to a
// variable, `set -e` -- is whatever `let` already did. Nothing new evaluates
// arithmetic, which is the point; a second evaluator would be a second set of
// answers.
//
// The status is the inverse of the value, which `let` already implements and which
// is what makes `((i < 10))` usable as a condition: measured, `((1))` exits 0 and
// `((0))` exits 1.

// arithmeticCommandEnd reports the index of the final `)` of an `((expr))` starting
// at index, or 0 when there is not one there.
//
// Built on arithmeticExpansionEnd, the same scan `$((` uses, so the two forms agree
// about where an expression ends -- including that a nested parenthesis does not
// close it.
func arithmeticCommandEnd(line string, index int) int {
	if index+3 >= len(line) || line[index] != '(' || line[index+1] != '(' {
		return 0
	}
	end, ok := arithmeticExpansionEnd(line, index+2)
	if !ok {
		return 0
	}
	return end
}

// arithmeticCommandText is the expression between the parentheses.
func arithmeticCommandText(line string, index, end int) string {
	return line[index+2 : end-1]
}

// arithmeticCommandTokens is the two words `((expr))` becomes: `let` and the expression.
//
// The expression goes in single-quoted, so its own blanks and stars are data --
// `(( i < 10 ))` is one argument to let, and the `*` in `(( a * b ))` is not a directory
// listing.
func arithmeticCommandTokens(line string, index, end int) []shellToken {
	text := arithmeticCommandText(line, index, end)
	return []shellToken{
		{kind: tokenWord, value: "let", parsed: &word{parts: []wordPart{{kind: wordPartLiteral, text: "let"}}}},
		{kind: tokenWord, value: text, parsed: &word{
			parts: []wordPart{{kind: wordPartLiteral, text: text, quote: quoteSingle}},
		}},
	}
}
