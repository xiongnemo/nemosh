package runtime

import "strings"

func parseTypedWord(result word) word {
	if len(result.parts) == 0 {
		result.parts = []wordPart{{kind: wordPartLiteral}}
	}
	return result
}

func parameterEnd(text string, start int) int {
	if start < len(text) && text[start] == '{' {
		if close := strings.IndexByte(text[start+1:], '}'); close >= 0 {
			return start + close + 2
		}
	}
	if start < len(text) && strings.ContainsRune("@*#?$!-", rune(text[start])) {
		return start + 1
	}
	if start < len(text) && text[start] >= '0' && text[start] <= '9' {
		return start + 1
	}
	end := start
	for end < len(text) && isNameByte(text[end]) {
		end++
	}
	if end == start {
		return start
	}
	return end
}

func containsError(err, target error) bool {
	return strings.Contains(err.Error(), target.Error())
}
