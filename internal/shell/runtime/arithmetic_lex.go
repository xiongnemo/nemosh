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
			if (operator == "++" || operator == "--") && !steps(tokens, expression[index+2:]) {
				operator = operator[:1]
			}
			tokens = append(tokens, operator)
			index += len(operator)
			continue
		}
		tokens = append(tokens, expression[index:index+1])
		index++
	}
	return tokens
}

// steps reports whether a `++` or `--` with rest after it steps a variable: the one just before
// it, or one it comes before, blanks allowed between. Otherwise it is two signs, as both
// references read it: `0++1` is 0 + +1, and `(a)+++3` is (a) + + + 3. It was an increment of
// nothing, `unexpected "++"`.
func steps(before []string, rest string) bool {
	if len(before) > 0 && isArithmeticName(before[len(before)-1]) {
		return true
	}
	rest = strings.TrimLeft(rest, " \t\n")
	return rest != "" && (rest[0] == '_' || rest[0] >= 'a' && rest[0] <= 'z' || rest[0] >= 'A' && rest[0] <= 'Z')
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
	// was. `@` is a digit there too, base 64's 62nd.
	if end > start && end < len(expression) && expression[end] == '#' && isDigits(expression[start:end]) {
		end++
		for end < len(expression) && (isNameByte(expression[end]) || expression[end] == '@') {
			end++
		}
	}
	// `a[i]` is one word, subscript and all -- `$(( a[0] + a[2] ))`, and the counting
	// idiom `(( count[$k]++ ))`. The `[` was a token of its own, `unexpected "["`. The
	// subscript is left as text; resolving it is the element lookup's business, which
	// knows whether the name is indexed or keyed.
	if end > start && end < len(expression) && expression[end] == '[' && isVariableName(expression[start:end]) {
		if close := matchingBracket(expression, end); close > end {
			end = close + 1
		}
	}
	return end
}

// matchingBracket is the index of the `]` that closes the `[` at open, or -1.
func matchingBracket(expression string, open int) int {
	depth := 0
	for index := open; index < len(expression); index++ {
		switch expression[index] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

// strayArithmeticToken is a character tokenizeArithmetic could not classify: no name, number
// or operator, and not the comma either. A `#` that is not a base's is one.
func strayArithmeticToken(token string) bool {
	return len(token) == 1 && token != "," && !isNameByte(token[0]) && matchArithmeticOperator(token) == ""
}

func matchArithmeticOperator(rest string) string {
	for _, operator := range arithmeticOperators {
		if strings.HasPrefix(rest, operator) {
			return operator
		}
	}
	return ""
}
