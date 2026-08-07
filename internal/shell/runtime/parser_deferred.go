package runtime

import (
	"fmt"
	"strings"
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
		if char == '(' || char == ')' {
			return fmt.Errorf("unsupported syntax: grouping")
		}
		// `;` is absent from this set: splitSequentialSegments cut the line at
		// every top-level separator before the line reached here, so one that
		// survives belongs to a construct this scan already stepped over.
		if char == '{' || char == '}' {
			return fmt.Errorf("unsupported syntax: %c", char)
		}
	}
	fields := strings.Fields(line)
	if len(fields) > 0 && fields[0] == "function" {
		return fmt.Errorf("unsupported syntax: function")
	}
	return nil
}
