package runtime

import (
	"slices"
	"strings"
)

// lexedOperator is the operator at the start of input, and its width, or a width of 0 when
// there is none there.
//
// Inside `[[ ]]` the command operators are the condition's own, as bash's lexer reads them
// there: `&&`, `||`, `<`, `>` and the parentheses end the word before them and are words of
// their own, which the condition's parser takes for operators when they are written plainly
// -- `[[ x||! (1 == 2)&&(2 == 2)]]`, `[[ b>a ]]`. `<` and `>` there are no redirections, and
// `&&` and `||` end no command. A `;`, a `&` and a `|` alone are not read here: the first
// ends the command before the lexer sees it, and the others are a regular expression's. The
// parentheses of a regular expression never reach here: the lexer reads them as a group of
// the word, as it does an extended pattern's, so a `)` that does reach here after `=~` is one
// the expression did not open, and closes a group of the condition. See regexWordOpens.
//
// The caller passes inCondition false for the operator after a word `]]`, which closes the
// conditional: `[[ a ]]||`, `[[ a ]]>/dev/null`.
func lexedOperator(input string, inCondition bool) (tokenKind, int) {
	if !inCondition {
		return activeOperator(input)
	}
	switch {
	case strings.HasPrefix(input, "&&"), strings.HasPrefix(input, "||"):
		return tokenWord, 2
	case strings.IndexByte("()<>", input[0]) >= 0:
		return tokenWord, 1
	}
	return tokenWord, 0
}

// regexWordOpens reports a `(` in the operand of `=~`, the word being read after it, which
// opens a group of the regular expression: bash reads a blank or a `|` inside one as the
// expression's own, so `(a  b)` is one word. It was two, the first an unbalanced `(a`, and a
// `)` after the expression was taken into it: in `[[ (x =~ x) ]]` it was `x)`.
func regexWordOpens(char byte, inCondition bool, tokens []shellToken) bool {
	last := len(tokens) - 1
	return char == '(' && inCondition && last >= 0 && tokens[last].kind == tokenWord && tokens[last].value == "=~"
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
	// Written plainly, as a reserved word is: `[[ x == "]]" ]]` is not over at its quotes.
	if token.kind != tokenWord || token.parsed == nil || !isUnquotedLiteralWord(*token.parsed) {
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
