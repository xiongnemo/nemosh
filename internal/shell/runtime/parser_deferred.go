package runtime

import (
	"fmt"
)

func rejectDeferredSyntax(line string) error {
	var quote byte
	substitutions := 0
	parameterBraces := 0
	escaped := false
	for index := 0; index < len(line); index++ {
		char := line[index]
		if escaped {
			escaped = false
			continue
		}
		if char == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if char == '\'' && quote != '"' {
			if quote == '\'' {
				quote = 0
			} else {
				quote = '\''
			}
			continue
		}
		if char == '"' && quote != '\'' {
			if quote == '"' {
				quote = 0
			} else {
				quote = '"'
			}
			continue
		}
		if quote != 0 {
			continue
		}
		// An arithmetic expansion is stepped over whole. Counting it as a
		// substitution would leave its inner `(` to be read as a grouping this
		// scan does not allow.
		if char == '$' && index+2 < len(line) && line[index+1] == '(' && line[index+2] == '(' {
			if end, ok := arithmeticExpansionEnd(line, index+3); ok {
				index = end
				continue
			}
		}
		if char == '$' && index+1 < len(line) && line[index+1] == '(' {
			substitutions++
			index++
			continue
		}
		if char == ')' && substitutions > 0 {
			substitutions--
			continue
		}
		if char == '$' && index+1 < len(line) && line[index+1] == '{' {
			parameterBraces++
			index++
			continue
		}
		if char == '}' && parameterBraces > 0 {
			parameterBraces--
			continue
		}
		if char == '(' {
			// An array assignment's parentheses belong to a word. This layer refuses
			// grouping outright, so without the test `a=(one two)` came back as
			// "unsupported syntax: grouping".
			if end, ok := arrayAssignmentSpan(line, index, line[:index]); ok {
				index = end
				continue
			}
			// And an arithmetic command's belong to itself. Fifth layer, which is
			// what the count in array.go is about: a construct spelled with a
			// parenthesis has to be recognised by every scan that has an opinion
			// about parentheses, and there are five of them.
			if end := arithmeticCommandEnd(line, index); end > 0 {
				index = end
				continue
			}
		}
		// A pattern group is not grouping: `[[ x == @(a|b) ]]` and a case pattern both
		// carry one, and this scan refuses parentheses outright.
		if extendedGroupOpensAt(line, index) {
			index = skipBalancedParens(line, index) - 1
			continue
		}
		if char == '(' || char == ')' {
			return fmt.Errorf("unsupported syntax: grouping")
		}
		// `;` is absent from this set: splitSequentialSegments cut the line at
		// every top-level separator before the line reached here, so one that
		// survives belongs to a construct this scan already stepped over.
		// Only a brace in command position is the reserved word. One inside or
		// after a word is an ordinary character and belongs to the operand:
		// `echo a}b`, `echo {}`, `echo x{1}y`.
		if (char == '{' || char == '}') && braceDelimiterAt(line, index, char) {
			return fmt.Errorf("unsupported syntax: %c", char)
		}
	}
	// The `function` keyword used to be refused here. It is a function definition now,
	// recognised before this scan runs; see parser_function.go.
	return nil
}
