package runtime

import (
	"errors"
	"strings"
)

func splitWords(line string) ([]string, error) {
	var args []string
	var current strings.Builder
	inSingle := false
	inDouble := false
	escaped := false
	pendingDollar := false
	commandDepth := 0
	flush := func() {
		if current.Len() > 0 {
			args = append(args, current.String())
			current.Reset()
		}
	}
	for _, r := range line {
		if escaped {
			current.WriteRune(r)
			escaped = false
			pendingDollar = false
			continue
		}
		if commandDepth > 0 {
			if pendingDollar && r == '(' {
				commandDepth++
				current.WriteRune(r)
				pendingDollar = false
				continue
			}
			if r == ')' {
				commandDepth--
				current.WriteRune(r)
				pendingDollar = false
				continue
			}
			current.WriteRune(r)
			pendingDollar = r == '$'
			continue
		}
		switch {
		case r == '\\' && !inSingle:
			escaped = true
			pendingDollar = false
		case r == '\'' && !inDouble:
			inSingle = !inSingle
			pendingDollar = false
		case r == '"' && !inSingle:
			inDouble = !inDouble
			pendingDollar = false
		case pendingDollar && r == '(' && !inSingle:
			commandDepth++
			current.WriteRune(r)
			pendingDollar = false
		case r == ')' && commandDepth > 0 && !inSingle:
			commandDepth--
			current.WriteRune(r)
			pendingDollar = false
		case (r == ' ' || r == '\t') && !inSingle && !inDouble && commandDepth == 0:
			flush()
			pendingDollar = false
		default:
			current.WriteRune(r)
			pendingDollar = r == '$' && !inSingle
		}
	}
	if escaped {
		current.WriteRune('\\')
	}
	if inSingle || inDouble {
		return nil, errors.New("unterminated quote")
	}
	flush()
	return args, nil
}
