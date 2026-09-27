package applets

import (
	"strconv"
	"strings"
)

// printf's numeric operands, read as C's strtoll and strtoull read them, which is how
// busybox's printf reads them: blanks first, a sign, 0x for hex or a leading 0 for octal,
// and digits to the end -- nothing after them, not even a blank. `' -123 '` was -123, and is
// an invalid number as in busybox. A leading quote makes the operand the code of the
// character after it, which POSIX specifies: `printf '%d' "'A"` is 65.
//
// An unsigned conversion takes the whole unsigned range, and a negative number as the
// same bits: `%u` of 18446744073709551615 is that number, and `%x` of -42 is
// ffffffffffffffd6. Both were refused or printed with a minus sign.

// printfSigned is the operand of %d and %i.
func printfSigned(operand string) (int64, error) {
	magnitude, negative, empty, ok := printfDigits(operand)
	switch {
	case empty:
		return 0, nil
	case !ok, !negative && magnitude > 1<<63-1, negative && magnitude > 1<<63:
		return 0, errPrintfNumber{operand: operand}
	case negative:
		return -int64(magnitude-1) - 1, nil
	}
	return int64(magnitude), nil
}

// printfUnsigned is the operand of %u, %o, %x and %X. A negative one has to fit the signed
// range, as it does in busybox, and is then taken as its bits.
func printfUnsigned(operand string) (uint64, error) {
	magnitude, negative, empty, ok := printfDigits(operand)
	switch {
	case empty:
		return 0, nil
	case !ok, negative && magnitude > 1<<63:
		return 0, errPrintfNumber{operand: operand}
	case negative:
		return ^magnitude + 1, nil
	}
	return magnitude, nil
}

// printfDigits reads an operand's sign and magnitude, and reports an operand that is empty
// and one that is not a number.
func printfDigits(operand string) (uint64, bool, bool, bool) {
	text := strings.TrimLeft(operand, " \t\n\v\f\r")
	if text == "" {
		return 0, false, true, true
	}
	if code, ok := printfCharacterCode(text); ok {
		if code < 0 {
			return uint64(-code), true, false, true
		}
		return uint64(code), false, false, true
	}
	negative := false
	if text[0] == '+' || text[0] == '-' {
		negative, text = text[0] == '-', text[1:]
	}
	base := 10
	switch {
	case len(text) > 1 && text[0] == '0' && (text[1] == 'x' || text[1] == 'X'):
		base, text = 16, text[2:]
	case len(text) > 1 && text[0] == '0':
		base, text = 8, text[1:]
	}
	// ParseUint takes no sign and no underscore at a base other than zero, which leaves it
	// exactly strtoull's digits.
	if text == "" || text[0] == '+' || text[0] == '-' {
		return 0, false, false, false
	}
	magnitude, err := strconv.ParseUint(text, base, 64)
	if err != nil {
		return 0, false, false, false
	}
	return magnitude, negative, false, true
}
