package runtime

import "strings"

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
	if input[0] == '|' {
		return tokenPipe, 1
	}
	if input[0] == '<' || input[0] == '>' {
		return tokenRedirect, redirectTokenWidth(input)
	}
	return tokenWord, 0
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
