package applets

import (
	"fmt"
	"math/big"
	"strings"
)

// Reading and writing numbers in bases other than ten, which is what `ibase` and `obase` are
// for.
//
// Two rules here are surprising enough to be worth stating, and both were measured:
//
//   - **A digit too large for the input base is clamped, not refused.** `ibase=8; 19` is 15,
//     because the `9` becomes a `7`. That is POSIX's rule and both references follow it; it
//     exists so that `ibase=16; 1F` and `ibase=16; 1f` can both work without the lexer
//     knowing the base.
//   - **A leading zero is not printed.** `0.5` comes back as `.5`. Every bc does this, and a
//     script comparing output has to expect it.
//
// Long output is broken with a backslash, as the references break it, because bc's output
// really is meant to be read by a person.

// decimalLineWidth is how many digits go on a line before the backslash. Measured: the
// references' first line of `2^300` is 69 characters, of which the last is the backslash.
const decimalLineWidth = 68

// parseDecimalDigits reads a number written in `base`, with an optional fractional part.
func parseDecimalDigits(text string, base int) (bigDecimal, error) {
	whole, fraction, hasFraction := strings.Cut(text, ".")
	value := big.NewInt(0)
	radix := big.NewInt(int64(base))
	for _, digit := range whole {
		place, ok := decimalDigitValue(digit, base)
		if !ok {
			return bigDecimal{}, fmt.Errorf("invalid character '%c' in a number", digit)
		}
		value.Add(value.Mul(value, radix), big.NewInt(int64(place)))
	}
	if !hasFraction {
		return bigDecimal{unscaled: value}, nil
	}
	// The fraction is read as an integer over base^n and then expressed in decimal, which
	// is the only way a base-16 fraction can land on a decimal scale at all.
	numerator := big.NewInt(0)
	for _, digit := range fraction {
		place, ok := decimalDigitValue(digit, base)
		if !ok {
			return bigDecimal{}, fmt.Errorf("invalid character '%c' in a number", digit)
		}
		numerator.Add(numerator.Mul(numerator, radix), big.NewInt(int64(place)))
	}
	scale := len(fraction)
	denominator := new(big.Int).Exp(radix, big.NewInt(int64(scale)), nil)
	lifted := new(big.Int).Mul(numerator, powerOfTen(scale))
	lifted.Quo(lifted, denominator)
	whole10 := bigDecimal{unscaled: value}.rescale(scale)
	return addDecimal(whole10, bigDecimal{unscaled: lifted, scale: scale}), nil
}

// decimalDigitValue answers a digit's value, clamped to the base's largest digit.
func decimalDigitValue(digit rune, base int) (int, bool) {
	var value int
	switch {
	case digit >= '0' && digit <= '9':
		value = int(digit - '0')
	case digit >= 'A' && digit <= 'F':
		value = int(digit-'A') + 10
	case digit >= 'a' && digit <= 'f':
		value = int(digit-'a') + 10
	default:
		return 0, false
	}
	if value >= base {
		// Clamped rather than refused -- see the file comment.
		return base - 1, true
	}
	return value, true
}

// formatDecimal writes a number in `base`, wrapped the way bc wraps it.
func formatDecimal(value bigDecimal, base int) (string, error) {
	if base < 2 || base > 16 {
		// Above sixteen POSIX prints each digit as a space-separated decimal group, which
		// is a different output format rather than a different alphabet. Refused by name
		// instead of approximated.
		return "", fmt.Errorf("obase must be between 2 and 16")
	}
	text, err := decimalDigits(value, base)
	if err != nil {
		return "", err
	}
	return wrapDecimalOutput(text), nil
}

func decimalDigits(value bigDecimal, base int) (string, error) {
	sign := ""
	if value.sign() < 0 {
		sign = "-"
		value = negateDecimal(value)
	}
	if value.isZero() {
		// A zero is `0` whatever its scale: `1.50 - 1.5` prints 0, not .00. Measured three
		// ways in both references, and it is the one place the scale does not show.
		return "0", nil
	}
	whole := new(big.Int).Quo(value.integer(), powerOfTen(value.scale))
	out := sign + whole.Text(base)
	if value.scale == 0 {
		return strings.ToUpper(out), nil
	}
	fraction := new(big.Int).Rem(value.integer(), powerOfTen(value.scale))
	digits, err := fractionDigits(fraction, value.scale, base)
	if err != nil {
		return "", err
	}
	if digits == "" {
		return strings.ToUpper(out), nil
	}
	if whole.Sign() == 0 {
		// The leading zero goes: bc prints `.5`, not `0.5`.
		return strings.ToUpper(sign + "." + digits), nil
	}
	return strings.ToUpper(out + "." + digits), nil
}

// fractionDigits converts the fractional part by repeated multiplication, which is how a
// decimal fraction becomes a base-N one.
//
// As many digits as the decimal scale asked for, which keeps `obase=16; scale=4` from
// running forever on a fraction that has no finite representation in the new base.
func fractionDigits(fraction *big.Int, scale, base int) (string, error) {
	if fraction.Sign() == 0 {
		return strings.Repeat("0", scale), nil
	}
	var digits strings.Builder
	remainder := new(big.Int).Set(fraction)
	divisor := powerOfTen(scale)
	radix := big.NewInt(int64(base))
	for index := 0; index < scale && remainder.Sign() != 0; index++ {
		remainder.Mul(remainder, radix)
		digit := new(big.Int).Quo(remainder, divisor)
		remainder.Rem(remainder, divisor)
		digits.WriteString(digit.Text(base))
	}
	// Padded to the full scale rather than stopped when the remainder runs out: a scale of
	// 4 means four digits, so `2^-2` is `.2500` and not `.25`. Dropping them would make the
	// scale invisible in exactly the case a reader set it to see.
	text := digits.String()
	if len(text) < scale {
		text += strings.Repeat("0", scale-len(text))
	}
	return text, nil
}

// wrapDecimalOutput breaks a long number with a trailing backslash, as the references do.
//
// Not cosmetic: `2^300` is 91 digits, and a terminal that soft-wraps it makes the reader
// count characters to find out whether two runs agree.
func wrapDecimalOutput(text string) string {
	if len(text) <= decimalLineWidth {
		return text
	}
	var out strings.Builder
	for len(text) > decimalLineWidth {
		out.WriteString(text[:decimalLineWidth])
		out.WriteString("\\\n")
		text = text[decimalLineWidth:]
	}
	out.WriteString(text)
	return out.String()
}
