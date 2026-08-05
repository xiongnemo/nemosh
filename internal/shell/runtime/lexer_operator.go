package runtime

import "strings"

func activeOperator(input string) (tokenKind, int) {
	if strings.HasPrefix(input, "&&") {
		return tokenAndIf, 2
	}
	if strings.HasPrefix(input, "||") {
		return tokenOrIf, 2
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
