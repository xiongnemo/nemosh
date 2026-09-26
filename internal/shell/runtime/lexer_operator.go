package runtime

import (
	"slices"
	"strings"
)

// lexedOperator is the operator at the start of input, and its width, or a width of 0 when
// there is none there.
//
// Inside `[[ ]]` none of the command operators are one, and its own parentheses are, as
// words of their own: `[[ (a == a) ]]` is five words between the brackets, which is how the
// condition parser reads a group. Not in the operand of `=~`, whose parentheses belong to
// the regular expression -- the word being read then is the one after the `=~`.
func lexedOperator(input string, inCondition bool, tokens []shellToken) (tokenKind, int) {
	if !inCondition {
		return activeOperator(input)
	}
	afterRegex := len(tokens) > 0 && tokens[len(tokens)-1].kind == tokenWord && tokens[len(tokens)-1].value == "=~"
	if (input[0] == '(' || input[0] == ')') && !afterRegex {
		return tokenWord, 1
	}
	return tokenWord, 0
}

// operatorToken is the token for an operator's text. A condition's parenthesis is a word, and
// a word carries its parsed form.
func operatorToken(kind tokenKind, text string) shellToken {
	token := shellToken{kind: kind, value: text}
	if kind == tokenWord {
		token.parsed = &word{parts: []wordPart{{kind: wordPartLiteral, text: text}}}
	}
	return token
}

func activeOperator(input string) (tokenKind, int) {
	if strings.HasPrefix(input, "&&") {
		return tokenAndIf, 2
	}
	if strings.HasPrefix(input, "||") {
		return tokenOrIf, 2
	}
	// `&>file` and `&>>file` send both stdout and stderr, and they have to be
	// tested before the bare `&`: otherwise `cmd &> log` is a background `cmd`
	// followed by a redirect of nothing, which is a different program.
	if strings.HasPrefix(input, "&>>") {
		return tokenRedirect, 3
	}
	if strings.HasPrefix(input, "&>") {
		return tokenRedirect, 2
	}
	if input[0] == '&' {
		return tokenBackground, 1
	}
	if strings.HasPrefix(input, "|&") {
		return tokenPipeStderr, 2
	}
	if input[0] == '|' {
		return tokenPipe, 1
	}
	if input[0] == '<' || input[0] == '>' {
		return tokenRedirect, redirectTokenWidth(input)
	}
	return tokenWord, 0
}

// expandPipeStderr turns each `|&` into what it means, `2>&1 |`: the redirection onto the
// command before it, then the pipe. So the pipeline and the redirection code see only the
// long form and nothing downstream has to know the short one. `a |& b` is bash's; it was a
// pipe followed by a background `&`, and a syntax error. Each new token keeps the offset
// of the `|&` it came from.
func expandPipeStderr(tokens []shellToken, starts []int) ([]shellToken, []int) {
	if !slices.ContainsFunc(tokens, func(token shellToken) bool { return token.kind == tokenPipeStderr }) {
		return tokens, starts
	}
	var expandedTokens []shellToken
	var expandedStarts []int
	for index, token := range tokens {
		if token.kind != tokenPipeStderr {
			expandedTokens = append(expandedTokens, token)
			expandedStarts = append(expandedStarts, starts[index])
			continue
		}
		one := word{parts: []wordPart{{kind: wordPartLiteral, text: "1"}}}
		expandedTokens = append(expandedTokens,
			shellToken{kind: tokenRedirect, value: "2>&"},
			shellToken{kind: tokenWord, value: "1", parsed: &one},
			shellToken{kind: tokenPipe, value: "|"})
		expandedStarts = append(expandedStarts, starts[index], starts[index], starts[index])
	}
	return expandedTokens, expandedStarts
}

func redirectTokenWidth(input string) int {
	if strings.HasPrefix(input, "<<<") {
		return 3
	}
	if strings.HasPrefix(input, "<<-") {
		return 3
	}
	if len(input) > 1 {
		switch input[:2] {
		case "<&", ">&", ">>", "<>", ">|", "<<":
			return 2
		}
	}
	return 1
}

// conditionAfterToken reports whether the scan is inside `[[ ]]` once token has been
// appended, which the lexer needs because inside a condition `&&`, `||`, `<` and `>` are
// not operators. Here rather than in the loop that calls it: lexer.go is at its line
// ceiling, and this is the one question in it that is about a token rather than a byte.
//
// `[[` counts only at the start of a command, because `echo [[` is two ordinary words.
func conditionAfterToken(inCondition bool, token shellToken, tokens []shellToken) bool {
	if token.kind != tokenWord {
		return inCondition
	}
	switch token.value {
	case "[[":
		return len(tokens) == 0 || tokens[len(tokens)-1].kind != tokenWord
	case "]]":
		return false
	}
	return inCondition
}

func isRedirectToken(value string) bool {
	index := 0
	for index < len(value) && value[index] >= '0' && value[index] <= '9' {
		index++
	}
	return index < len(value) && (value[index] == '<' || value[index] == '>')
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for index := range len(value) {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return true
}

func tokenValues(tokens []shellToken) []string {
	values := make([]string, len(tokens))
	for index, token := range tokens {
		values[index] = token.value
	}
	return values
}
