package applets

import (
	"strconv"
	"strings"
)

// A `*` for the width or the precision takes it from the next operand: `printf '%*d' 5 42`
// right-aligns 42 in five columns, and `%.*f` rounds to as many places as an operand says.
// POSIX leaves it to the implementation and both references have it; here it was an
// invalid conversion specification. A negative width means left-aligned and a negative
// precision means none, as in C.

// printfNumberOrStar steps over a run of digits or a single `*`.
func printfNumberOrStar(rest string, index int) int {
	if index < len(rest) && rest[index] == '*' {
		return index + 1
	}
	for index < len(rest) && rest[index] >= '0' && rest[index] <= '9' {
		index++
	}
	return index
}

// resolvePrintfStars writes each `*` in a specification as the operand it stands for.
func resolvePrintfStars(spec string, next func() string) (string, error) {
	if !strings.Contains(spec, "*") {
		return spec, nil
	}
	var out strings.Builder
	var firstErr error
	for index := 0; index < len(spec); index++ {
		if spec[index] != '*' {
			out.WriteByte(spec[index])
			continue
		}
		value, err := printfInteger(next())
		if err != nil && firstErr == nil {
			firstErr = err
		}
		precision := index > 0 && spec[index-1] == '.'
		switch {
		case value < 0 && precision:
			// No precision at all: take back the dot this star followed.
			written := out.String()
			out.Reset()
			out.WriteString(strings.TrimSuffix(written, "."))
			continue
		case value < 0:
			out.WriteByte('-')
			value = -value
		}
		out.WriteString(strconv.FormatInt(value, 10))
	}
	return out.String(), firstErr
}
