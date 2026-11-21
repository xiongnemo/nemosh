package applets

import (
	"math"
	"strconv"
	"strings"
)

// printf's floating operands, read as C's strtod reads them, which both references do: blanks
// first, a sign, then inf, infinity or nan in any case, a hex number with or without its p
// exponent, or decimal digits with a point and an e exponent, each optional where C has it so.
// Go's ParseFloat reads Go's literals instead, so `1_0` was 10, `0x1.8` and `0x10` no number,
// and a blank before or after the number was trimmed away.

// printfFloat is the operand of %e, %f, %g and their capitals: the number, and an error when
// the operand is not one, whole. An operand given empty is zero, as busybox has it.
func printfFloat(operand string) (float64, error) {
	if operand == "" {
		return 0, nil
	}
	value, length, err := strtod(operand)
	if length == 0 || length < len(operand) || err != nil {
		return value, errPrintfNumber{operand: operand}
	}
	return value, nil
}

// strtod is the value of the longest prefix of text that is a number, its length, which is 0
// when there is none, and the range error for one too large or too small.
func strtod(text string) (float64, int, error) {
	start := len(text) - len(strings.TrimLeft(text, " \t\n\v\f\r"))
	at := start
	negative := at < len(text) && text[at] == '-'
	if at < len(text) && (text[at] == '+' || text[at] == '-') {
		at++
	}
	rest := strings.ToLower(text[at:])
	for _, word := range []string{"infinity", "inf", "nan"} {
		if strings.HasPrefix(rest, word) {
			value := math.NaN()
			if word != "nan" {
				value = math.Inf(1)
				if negative {
					value = math.Inf(-1)
				}
			}
			return value, at + len(word), nil
		}
	}
	hex := len(text) > at+1 && text[at] == '0' && text[at+1]|0x20 == 'x'
	digit, exponent, mantissa := isDecimalDigit, byte('e'), at
	if hex {
		digit, exponent, mantissa = isHexDigit, 'p', at+2
	}
	end := scanDigits(text, mantissa, digit)
	if end < len(text) && text[end] == '.' {
		end = scanDigits(text, end+1, digit)
	}
	if end == mantissa || end == mantissa+1 && text[mantissa] == '.' {
		if hex {
			// `0x` with nothing after it is the 0 before the x.
			return 0, at + 1, nil
		}
		return 0, 0, nil
	}
	if end < len(text) && text[end]|0x20 == exponent {
		power := end + 1
		if power < len(text) && (text[power] == '+' || text[power] == '-') {
			power++
		}
		if power < len(text) && isDecimalDigit(text[power]) {
			end = scanDigits(text, power, isDecimalDigit)
		}
	}
	number := text[start:end]
	if hex && !strings.ContainsAny(number, "pP") {
		number += "p0"
	}
	value, err := strconv.ParseFloat(number, 64)
	return value, end, err
}

func scanDigits(text string, at int, digit func(byte) bool) int {
	for at < len(text) && digit(text[at]) {
		at++
	}
	return at
}

func isDecimalDigit(c byte) bool { return c >= '0' && c <= '9' }
