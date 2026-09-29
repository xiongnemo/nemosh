package applets

import (
	"math"
	"strconv"
	"strings"
)

// cFloat is C's %e %E %f %g %G: six digits unless the precision says, the exponent in two
// digits at least, %g's trailing zeros dropped, and inf and nan as C spells them. Go's %g is
// the shortest form that reads back instead, and msvcrt writes 1.#INF000e+000.
func cFloat(spec cSpec, verb byte, value float64) string {
	sign := ""
	switch {
	case math.Signbit(value):
		sign = "-"
	case spec.plus:
		sign = "+"
	case spec.space:
		sign = " "
	}
	finite := !math.IsInf(value, 0) && !math.IsNaN(value)
	body := "nan"
	switch {
	case math.IsInf(value, 0):
		body = "inf"
	case finite:
		body = cFloatDigits(spec, verb|0x20, math.Abs(value))
	}
	if verb == 'E' || verb == 'G' {
		body = strings.ToUpper(body)
	}
	return cPad(spec, sign, body, finite)
}

func cFloatDigits(spec cSpec, verb byte, value float64) string {
	precision := spec.precision
	if precision < 0 {
		precision = 6
	}
	switch verb {
	case 'f':
		text := strconv.FormatFloat(value, 'f', precision, 64)
		if spec.hash && precision == 0 {
			text += "."
		}
		return text
	case 'e':
		return cExponent(value, precision, spec.hash)
	}
	// %g is %e when the exponent is below -4 or not below the precision, and %f otherwise,
	// with as many digits in all as the precision.
	precision = max(precision, 1)
	exponent := 0
	if value != 0 {
		text := strconv.FormatFloat(value, 'e', precision-1, 64)
		exponent, _ = strconv.Atoi(text[strings.IndexByte(text, 'e')+1:])
	}
	text := ""
	if exponent < -4 || exponent >= precision {
		text = cExponent(value, precision-1, spec.hash)
	} else {
		text = strconv.FormatFloat(value, 'f', precision-1-exponent, 64)
		if spec.hash && !strings.Contains(text, ".") {
			text += "."
		}
	}
	if spec.hash {
		return text
	}
	mantissa, exponentPart := text, ""
	if at := strings.IndexByte(text, 'e'); at >= 0 {
		mantissa, exponentPart = text[:at], text[at:]
	}
	if strings.Contains(mantissa, ".") {
		mantissa = strings.TrimRight(strings.TrimRight(mantissa, "0"), ".")
	}
	return mantissa + exponentPart
}

// cExponent is %e's digits: d.ddde+dd, with a point and no digits after it for # with none.
func cExponent(value float64, precision int, hash bool) string {
	text := strconv.FormatFloat(value, 'e', precision, 64)
	if hash && precision == 0 {
		at := strings.IndexByte(text, 'e')
		text = text[:at] + "." + text[at:]
	}
	return text
}
