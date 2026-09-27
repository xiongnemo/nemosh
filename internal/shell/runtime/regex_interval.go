package runtime

import (
	"errors"
	"strings"
)

var errBraceWithoutInterval = errors.New("a { that begins no interval")

// posixIntervals holds an extended regular expression to POSIX's reading of `{`: it begins an
// interval, `{n}`, `{n,}` or `{n,m}`, and anything else is an invalid expression, as it is to
// busybox-w32 and to bash. Go's regexp takes such a `{` for a literal brace, so `[[ { =~ { ]]`
// was true where both references answer 2. `{,m}` is `{0,m}`, as busybox's library reads it
// and Go's would not. An escaped brace, and one in a bracket expression, is a brace.
func posixIntervals(source string) (string, error) {
	var out strings.Builder
	for index := 0; index < len(source); index++ {
		switch char := source[index]; {
		case char == '\\' && index+1 < len(source):
			out.WriteString(source[index : index+2])
			index++
			continue
		case char == '[':
			end := bracketExpressionEnd(source, index)
			out.WriteString(source[index:end])
			index = end - 1
			continue
		case char != '{':
			out.WriteByte(char)
			continue
		}
		close := strings.IndexByte(source[index:], '}')
		if close < 0 {
			return "", errBraceWithoutInterval
		}
		bound := source[index+1 : index+close]
		low, high, comma := strings.Cut(bound, ",")
		if !allDigits(high) || !allDigits(low) || low == "" && (!comma || high == "") {
			return "", errBraceWithoutInterval
		}
		if low == "" {
			low = "0"
		}
		out.WriteString("{" + low)
		if comma {
			out.WriteString("," + high)
		}
		out.WriteByte('}')
		index += close
	}
	return out.String(), nil
}

// bracketExpressionEnd is one past the `]` that closes the bracket expression at open, where a
// `]` first in it, after the `[` or `[^`, is a member, and `[:alpha:]` and its kind are whole.
// Without one it is the end of the text, which the compiler then refuses.
func bracketExpressionEnd(source string, open int) int {
	index := open + 1
	if index < len(source) && source[index] == '^' {
		index++
	}
	if index < len(source) && source[index] == ']' {
		index++
	}
	for ; index < len(source); index++ {
		if source[index] == '[' && index+1 < len(source) && strings.IndexByte(":.=", source[index+1]) >= 0 {
			if end := strings.Index(source[index+2:], string(source[index+1])+"]"); end >= 0 {
				index += end + 3
				continue
			}
		}
		if source[index] == ']' {
			return index + 1
		}
	}
	return len(source)
}

func allDigits(text string) bool {
	for index := 0; index < len(text); index++ {
		if text[index] < '0' || text[index] > '9' {
			return false
		}
	}
	return true
}
