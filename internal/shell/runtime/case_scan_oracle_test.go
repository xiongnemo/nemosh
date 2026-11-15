package runtime

import "strings"

// casePatternPosition and insideCase as they were written before caseScan and caseWords,
// reading the prefix from its first byte every time they are asked. Kept as the oracle the
// scans are held to (case_scan_test.go), and not to be changed except with the rules they
// encode: a rule added to the scans and not here is a rule the test cannot check.

func casePatternPositionAsWritten(prefix string) bool {
	depth := 0
	pattern := false
	subject := false
	ownIn := false
	var word strings.Builder
	endWord := func() {
		text := word.String()
		word.Reset()
		afterSubject := ownIn
		if text != "" {
			ownIn = false
		}
		switch {
		case text == "":
		case subject:
			subject = false
			ownIn = true
		case text == "case" && !pattern:
			depth++
			pattern = false
			subject = true
		case text == "in":
			if afterSubject {
				pattern = true
			}
		case text == "esac":
			if depth > 0 {
				depth--
			}
			pattern = false
		}
	}

	quote := byte(0)
	escaped := false
	for index := 0; index < len(prefix); index++ {
		char := prefix[index]
		if end := ansiQuoteClose(prefix, index); !escaped && quote == 0 && end >= 0 {
			word.WriteString(prefix[index : end+1])
			index = end
			continue
		}
		switch {
		case escaped:
			escaped = false
			word.WriteByte(char)
		case char == '\\' && quote != '\'':
			escaped = true
			word.WriteByte(char)
		case char == '\'' && quote != '"', char == '"' && quote != '\'':
			if quote == char {
				quote = 0
			} else if quote == 0 {
				quote = char
			}
			word.WriteByte(char)
		case quote != 0:
			word.WriteByte(char)
		case char == '$' && index+1 < len(prefix) && (prefix[index+1] == '(' || prefix[index+1] == '{'):
			end, ok := expansionEnd(prefix, index)
			if !ok {
				return false
			}
			word.WriteString(prefix[index : end+1])
			index = end
		case char == '(' && index+1 < len(prefix) && prefix[index+1] == '(':
			end := arithmeticCommandEnd(prefix, index)
			if end == 0 {
				return false
			}
			endWord()
			index = end
		case char == ';':
			endWord()
			if index+1 < len(prefix) && prefix[index+1] == ';' {
				index++
				if index+1 < len(prefix) && prefix[index+1] == '&' {
					index++
				}
				pattern = depth > 0
			}
		case char == ')':
			endWord()
			pattern = false
		case char == '(', char == '|', char == '&', char == ' ', char == '\t', char == '\n':
			endWord()
		default:
			word.WriteByte(char)
		}
	}
	endWord()
	return depth > 0 && pattern
}

func insideCaseAsWritten(text string) bool {
	depth := 0
	subject := false
	for _, field := range strings.Fields(text) {
		word := strings.Trim(field, ";&|(){}")
		switch {
		case subject:
			subject = false
		case word == "case":
			depth++
			subject = true
		case word == "esac":
			if depth > 0 {
				depth--
			}
		}
	}
	return depth > 0
}
