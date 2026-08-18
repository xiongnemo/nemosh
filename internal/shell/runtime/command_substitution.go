package runtime

func commandSubstitutionEnd(input string, bodyStart int) (int, bool) {
	quotes := []byte{0}
	escaped := false
	for index := bodyStart; index < len(input); index++ {
		char := input[index]
		if escaped {
			escaped = false
			continue
		}
		quote := quotes[len(quotes)-1]
		if quote == '\'' {
			if char == '\'' {
				quotes[len(quotes)-1] = 0
			}
			continue
		}
		if char == '\\' {
			escaped = true
			continue
		}
		if char == '\'' && quote == 0 {
			quotes[len(quotes)-1] = char
			continue
		}
		if char == '"' {
			if quote == '"' {
				quotes[len(quotes)-1] = 0
			} else if quote == 0 {
				quotes[len(quotes)-1] = char
			}
			continue
		}
		if char == '$' && index+1 < len(input) && input[index+1] == '(' && quote != '\'' {
			quotes = append(quotes, 0)
			index++
			continue
		}
		// `x=$(a=(1 2); echo ${a[0]})` -- the parenthesis of an array assignment
		// is part of a word, not a subshell, so its closing one must not be taken
		// for the end of the substitution. Without this the scan popped its own
		// nesting at `2)` and then reported the real `)` as missing. This is the
		// fifth scanner to need the rule; the other four are named in array.go.
		if char == '(' && quote == 0 {
			if end, ok := arrayAssignmentSpan(input, index, input[bodyStart:index]); ok {
				index = end
				continue
			}
		}
		if char == ')' && quote == 0 {
			quotes = quotes[:len(quotes)-1]
			if len(quotes) == 0 {
				return index, true
			}
		}
	}
	return 0, false
}
