package applets

import (
	"math"
	"strings"
)

// bash's printf reads a numeric operand with strtoimax, strtoumax or strtold, and writes what
// they read before they stopped, said as an invalid number when they stopped short of the end;
// one too large for the conversion is the largest it holds, said as out of range. `3abc` is 3,
// ` -42 ` is -42, `0x1g` is 1, 99999999999999999999 is 9223372036854775807 to %d. Measured
// against Git for Windows' bash 5.3. busybox writes zero for every one, as printf_number.go has
// it, which nemosh and sh keep.

// bashPrintfInteger is the operand of an integer conversion as bash reads it: its bits, a
// signed conversion's as two's complement, and the error to say, if any. One that is missing is
// zero and no error.
func bashPrintfInteger(operand string, given, unsigned bool) (uint64, error) {
	if !given {
		return 0, nil
	}
	if code, ok := printfCharacterCode(operand); ok {
		return uint64(code), nil
	}
	magnitude, overflow, negative, length := strtoint(operand)
	bits, ranged := magnitude, overflow
	switch {
	case unsigned && overflow:
		bits = math.MaxUint64
	case unsigned && negative:
		bits = -magnitude
	case negative && (overflow || magnitude > 1<<63):
		bits, ranged = 1<<63, true
	case negative:
		bits = -magnitude
	case overflow || magnitude > math.MaxInt64:
		bits, ranged = math.MaxInt64, true
	}
	if length == 0 || length < len(operand) {
		return bits, errPrintfNumber{operand: operand}
	}
	if ranged {
		return bits, errPrintfRange{operand: operand}
	}
	return bits, nil
}

// bashPrintfFloat is the operand of a floating conversion as bash reads it. One given empty is
// an invalid number to bash, where busybox reads it as zero.
func bashPrintfFloat(operand string, given bool) (float64, error) {
	if !given {
		return 0, nil
	}
	if code, ok := printfCharacterCode(operand); ok {
		return float64(code), nil
	}
	value, length, err := strtod(operand)
	if length == 0 || length < len(operand) {
		return value, errPrintfNumber{operand: operand}
	}
	if err != nil {
		return value, errPrintfRange{operand: operand}
	}
	return value, nil
}

// strtoint reads text as C's strtoimax and strtoumax read it, in base 0: blanks, a sign, then 0x
// and hex digits, 0 and octal ones, or decimal ones. It answers the magnitude, whether it is
// more than 64 bits hold, the sign, and how much of text it read, 0 when there was no digit.
func strtoint(text string) (uint64, bool, bool, int) {
	at := len(text) - len(strings.TrimLeft(text, " \t\n\v\f\r"))
	negative := at < len(text) && text[at] == '-'
	if at < len(text) && (text[at] == '+' || text[at] == '-') {
		at++
	}
	base := uint64(10)
	switch {
	case len(text) > at+2 && text[at] == '0' && text[at+1]|0x20 == 'x' && isHexDigit(text[at+2]):
		base, at = 16, at+2
	case at < len(text) && text[at] == '0':
		base = 8
	}
	var magnitude uint64
	overflow, start := false, at
	for ; at < len(text); at++ {
		digit, ok := digitValue(text[at])
		if !ok || digit >= base {
			break
		}
		if magnitude > (math.MaxUint64-digit)/base {
			overflow = true
		} else if !overflow {
			magnitude = magnitude*base + digit
		}
	}
	if at == start {
		return 0, false, false, 0
	}
	return magnitude, overflow, negative, at
}

// digitValue is a digit's value in any base up to 16.
func digitValue(c byte) (uint64, bool) {
	switch {
	case isDecimalDigit(c):
		return uint64(c - '0'), true
	case c|0x20 >= 'a' && c|0x20 <= 'f':
		return uint64(c|0x20-'a') + 10, true
	}
	return 0, false
}
