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
			continue
		}
		switch {
		case r == '\\' && !inSingle:
			escaped = true
		case r == '\'' && !inDouble:
			inSingle = !inSingle
		case r == '"' && !inSingle:
			inDouble = !inDouble
		case (r == ' ' || r == '\t') && !inSingle && !inDouble:
			flush()
		default:
			current.WriteRune(r)
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
