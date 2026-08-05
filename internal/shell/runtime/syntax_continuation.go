package runtime

func hasTrailingSyntaxOperator(line string) bool {
	quote := byte(0)
	escaped := false
	depth := 0
	trailing := false
	for index := 0; index < len(line); index++ {
		char := line[index]
		if escaped {
			escaped = false
			if depth == 0 {
				trailing = false
			}
			continue
		}
		if char == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if char == '\'' && quote != '"' || char == '"' && quote != '\'' {
			if quote == char {
				quote = 0
			} else if quote == 0 {
				quote = char
				if depth == 0 {
					trailing = false
				}
			}
			continue
		}
		if quote != 0 {
			continue
		}
		if char == '#' && depth == 0 && commentStarts(line, index) {
			break
		}
		if char == '$' && index+1 < len(line) && line[index+1] == '(' {
			if depth == 0 {
				trailing = false
			}
			depth++
			index++
			continue
		}
		if char == '(' || braceDelimiterAt(line, index, '{') {
			if depth == 0 {
				trailing = false
			}
			depth++
			continue
		}
		if (char == ')' || braceDelimiterAt(line, index, '}')) && depth > 0 {
			depth--
			continue
		}
		if depth != 0 || char == ' ' || char == '\t' {
			continue
		}
		if char == '<' || char == '>' {
			trailing = true
			index += redirectTokenWidth(line[index:]) - 1
			continue
		}
		if index+1 < len(line) && (line[index:index+2] == "&&" || line[index:index+2] == "||") {
			trailing = true
			index++
			continue
		}
		if char == '|' {
			trailing = true
			continue
		}
		trailing = false
	}
	return trailing
}
