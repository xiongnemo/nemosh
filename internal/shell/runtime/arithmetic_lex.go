package runtime

import "strings"

// Longest first, so `<<=` is not read as `<<` then `=`, and `<<` is not read as
// two `<`.
var arithmeticOperators = []string{
	"<<=", ">>=",
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
