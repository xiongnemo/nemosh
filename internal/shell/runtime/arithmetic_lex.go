package runtime

import "strings"

// Longest first, so `<<=` is not read as `<<` then `=`, and `<<` is not read as
// two `<`.
var arithmeticOperators = []string{
	"<<=", ">>=",
	// `++` and `--` before `+=` and `-=`, and both before the single characters:
	// otherwise `i++` lexes as `i`, `+`, `+` and the second plus has no operand,
	// which is exactly the "expression ended early" it used to report.
	"++", "--",
	// `#` is not an operator, but it has to be *absent* from this list for
	// `2#101` to stay one token. Recorded here because a reader adding operators
	// will wonder.

	// `**` before `*`, or `2**10` lexes as two multiplications with nothing
	// between them -- which is the "unexpected *" it used to report.
	"**",
	"<<", ">>", "<=", ">=", "==", "!=", "&&", "||",
	"+=", "-=", "*=", "/=", "%=", "&=", "^=", "|=",
	"+", "-", "*", "/", "%", "(", ")", "<", ">", "&", "^", "|", "!", "~", "?", ":", "=",
}

// arithmeticExpansionEnd finds the `))` that closes a `$((` whose body starts
// at bodyStart, and reports the index of the second `)`. Parentheses inside the
// expression nest, so the count is what ends it rather than the first `))`
// found -- `$(( (1+2) * 3 ))` closes at the very end.
func arithmeticExpansionEnd(input string, bodyStart int) (int, bool) {
	depth := 0
	for index := bodyStart; index+1 < len(input); index++ {
		switch input[index] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
				continue
			}
			if input[index+1] == ')' {
				return index + 1, true
			}
			return 0, false
		}
	}
	return 0, false
}

// tokenizeArithmetic splits an arithmetic expression into numbers, names, and
// operators. Whitespace only separates; it carries no meaning of its own.
// Anything it cannot classify comes back as a one-character token, which the
// parser then reports by name rather than silently skipping.
func tokenizeArithmetic(expression string) []string {
	var tokens []string
	for index := 0; index < len(expression); {
		char := expression[index]
		if char == ' ' || char == '\t' || char == '\n' {
			index++
			continue
		}
		if end := arithmeticWordEnd(expression, index); end > index {
			tokens = append(tokens, expression[index:end])
			index = end
			continue
		}
		if operator := matchArithmeticOperator(expression[index:]); operator != "" {
			tokens = append(tokens, operator)
			index += len(operator)
			continue
		}
		tokens = append(tokens, expression[index:index+1])
		index++
	}
	return tokens
}

// A word is a name or a number; 0x and 0 prefixes are left for ParseInt to
// read, so hexadecimal and octal work the way C spells them.
func arithmeticWordEnd(expression string, start int) int {
	end := start
	for end < len(expression) && (isNameByte(expression[end]) || expression[end] == 'x' || expression[end] == 'X') {
		end++
	}
	// `base#digits` is one word: `2#101` is 5, and `16#ff` is 255. Without this the
	// `#` ended the word and became a token of its own, which the parser reported as
	// `unexpected "#"`. Only after digits, so a `#` anywhere else is still whatever it
	// was.
	if end > start && end < len(expression) && expression[end] == '#' && isDigits(expression[start:end]) {
		end++
		for end < len(expression) && isNameByte(expression[end]) {
			end++
		}
	}
	return end
}

func matchArithmeticOperator(rest string) string {
	for _, operator := range arithmeticOperators {
		if strings.HasPrefix(rest, operator) {
			return operator
		}
	}
	return ""
}
