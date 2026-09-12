package applets

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// awk's value model, which is the whole game.
//
// A value is a string, a number, or a **strnum** -- and that third kind is what makes awk
// awk. A field, a `getline` result, a `-v` assignment, an `ARGV` element or an `FS`-split
// element that *looks* numeric compares **numerically**; a string literal never does. So
// with input `10 9`:
//
//	$1 < $2      is 0   -- both strnum, compared as numbers
//	"10" < "9"   is 1   -- both literals, compared as strings
//	x = "10"; x < 9 is 1 -- an assigned literal is still a literal
//	-v v=10; v < 9  is 0 -- a -v value is strnum
//
// Every line of that was measured against gawk 5.4.1 and busybox-w32 1.38.0, which agree
// on all forty-odd cases the tests cover. This cannot be retrofitted: get it wrong and
// every comparison in every program is subtly wrong, in a way whose failing tests are
// exactly the ones nobody thinks to write.
//
// Uninitialised is **both** `""` and `0`, which is why it is its own kind rather than an
// empty string: `u == 0` and `u == ""` are both true at once.

// awkValueKind is which of the three a value is, plus the uninitialised fourth.
type awkValueKind uint8

const (
	// awkUninit is the zero value on purpose, so a fresh variable needs no constructor.
	awkUninit awkValueKind = iota
	awkNumber
	awkString
	awkStrnum
)

type awkValue struct {
	kind   awkValueKind
	number float64
	text   string
	// looksNumeric is meaningful only for a strnum: whether the text is wholly a
	// number, blanks aside. Computed once at construction because every comparison
	// asks, and because the answer cannot change afterwards.
	looksNumeric bool
}

func awkNum(value float64) awkValue { return awkValue{kind: awkNumber, number: value} }
func awkStr(text string) awkValue   { return awkValue{kind: awkString, text: text} }

// awkBool is a comparison's result, which in awk is the number 1 or 0 rather than a
// separate type -- `print (1 < 2)` prints `1`.
func awkBool(value bool) awkValue {
	if value {
		return awkNum(1)
	}
	return awkNum(0)
}

// awkStrnumOf makes the kind that input produces: text that is compared as a number when
// it reads as one, and as a string when it does not.
func awkStrnumOf(text string) awkValue {
	value, whole := awkScanNumber(text)
	return awkValue{kind: awkStrnum, text: text, number: value, looksNumeric: whole}
}

// num is the value as a number.
//
// A string converts by its **numeric prefix** and never fails: `"12abc"` is 12, `"abc"`
// is 0, `""` is 0. That is POSIX's atof rule, and both references agree on every case.
func (v awkValue) num() float64 {
	switch v.kind {
	case awkNumber:
		return v.number
	case awkStrnum:
		return v.number
	case awkString:
		value, _ := awkScanNumber(v.text)
		return value
	}
	return 0
}

// str is the value as a string, rendering a number through convfmt.
func (v awkValue) str(convfmt string) string {
	switch v.kind {
	case awkNumber:
		return formatAwkNumber(v.number, convfmt)
	case awkString, awkStrnum:
		return v.text
	}
	return ""
}

// boolean is truthiness, and it follows the strnum rule too: a *field* holding `0` is
// false, while the string literal `"0"` is true because it is a non-empty string.
// Measured; both references agree.
func (v awkValue) boolean() bool {
	switch v.kind {
	case awkNumber:
		return v.number != 0
	case awkString:
		return v.text != ""
	case awkStrnum:
		if v.looksNumeric {
			return v.number != 0
		}
		return v.text != ""
	}
	return false
}

// comparesAsNumber reports whether this side would like a numeric comparison.
//
// Uninitialised counts, which is what makes `u == 0` true.
func (v awkValue) comparesAsNumber() bool {
	switch v.kind {
	case awkNumber, awkUninit:
		return true
	case awkStrnum:
		return v.looksNumeric
	}
	return false
}

// compareAwkValues answers -1, 0 or 1.
//
// **A string on either side forces a string comparison**, with the other side converted.
// That is the rule that makes `10 == "10.0"` false -- `"10"` against `"10.0"` -- while
// `10 == 10.0` is true and, with a field holding `10`, `$1 == 10.0` is true as well.
// Measured; the three answers differ and all three matter.
func compareAwkValues(left, right awkValue, convfmt string) int {
	if left.comparesAsNumber() && right.comparesAsNumber() {
		leftNumber, rightNumber := left.num(), right.num()
		switch {
		case leftNumber < rightNumber:
			return -1
		case leftNumber > rightNumber:
			return 1
		}
		return 0
	}
	return strings.Compare(left.str(convfmt), right.str(convfmt))
}

// formatAwkNumber renders a number the way awk prints one.
//
// **An integral value prints as an integer**, whatever CONVFMT or OFMT say -- `3.0` is
// `3` -- and everything else goes through the format. The integer form stops at what
// fits in an int64, which is where the two references part company: busybox prints
// `1e+19` past that point and gawk prints all twenty digits. busybox is the primary
// reference here and is also the more honest answer, because a float64 carries about
// seventeen significant digits and the rest of gawk's are artefacts of the binary
// representation rather than information.
//
// (busybox on Windows actually prints `1e+019`, a three-digit exponent from the MSVC
// runtime. That is a C library quirk rather than a specification, and Go's `1e+19` is
// what C99 and gawk's own %g would produce.)
// twoToThe63 is the bound every float-to-integer conversion here is written against, as a
// strict `<` rather than a `<=` against math.MaxInt64. The difference is a real bug rather
// than pedantry: `float64(math.MaxInt64)` rounds **up** to 2^63, so the `<=` form admitted
// 2^63 itself and `int64(2^63)` wraps to -9223372036854775808 -- `2^63` printed as a large
// negative number. MinInt64 is exactly representable, so the lower bound stays `>=`.
const twoToThe63 = 9223372036854775808.0

func formatAwkNumber(value float64, format string) string {
	// Infinity and NaN are spelled before anything else, because Go's fmt renders them
	// `+Inf` and `NaN`, which matches neither reference. gawk prints `+inf` and `-inf`;
	// busybox prints `1.#INF`, which is the MSVC runtime again rather than a rule. gawk
	// is followed, being the one that is not a platform artefact.
	switch {
	case math.IsInf(value, 1):
		return "+inf"
	case math.IsInf(value, -1):
		return "-inf"
	case math.IsNaN(value):
		return "nan"
	}
	if value == math.Trunc(value) && value >= math.MinInt64 && value < twoToThe63 {
		// -0 prints as 0, which both references do and which strconv would otherwise
		// render as "-0".
		if value == 0 {
			return "0"
		}
		return strconv.FormatInt(int64(value), 10)
	}
	// CONVFMT and OFMT are C format strings, and Go's fmt reads `%.6g` the same way.
	// A format awk would accept and Go would not is a stage-7 problem, when printf's
	// own machinery lands; until then a bad one renders as Go's %!verb, which is loud.
	return fmt.Sprintf(format, value)
}

// awkScanNumber reads the leading number of a string, and reports whether the *whole*
// string was one once blanks are set aside.
//
// The two answers are wanted together because they are the same scan: `num()` takes the
// prefix and strnum classification needs the "wholly" flag.
//
// **Hex is not a number.** `"0x10" + 0` is 0 and `"0x10"` is not a strnum. The references
// disagree here -- gawk says 0 and busybox says 16, because busybox hands the text to a
// strtod that accepts hex -- and gawk is followed because busybox contradicts *itself*:
// it declines to treat `0x10` as a strnum for comparison and then converts it as hex for
// arithmetic. POSIX says decimal. Recorded as a divergence in docs/support-matrix.md.
func awkScanNumber(text string) (float64, bool) {
	trimmed := strings.TrimLeft(text, " \t\n\r\v\f")
	end := awkNumberEnd(trimmed)
	if end == 0 {
		return 0, false
	}
	value, err := strconv.ParseFloat(trimmed[:end], 64)
	if err != nil {
		return 0, false
	}
	whole := strings.TrimRight(trimmed[end:], " \t\n\r\v\f") == ""
	return value, whole
}

// awkNumberEnd is how many bytes of a decimal number begin the text.
//
// Written out rather than handed to strconv.ParseFloat, which accepts three things awk
// does not: hex floats (`0x10p0`), `Inf` and `NaN`. A field reading `inf` is text.
func awkNumberEnd(text string) int {
	index := 0
	if index < len(text) && (text[index] == '+' || text[index] == '-') {
		index++
	}
	digits := 0
	for index < len(text) && text[index] >= '0' && text[index] <= '9' {
		index, digits = index+1, digits+1
	}
	if index < len(text) && text[index] == '.' {
		index++
		for index < len(text) && text[index] >= '0' && text[index] <= '9' {
			index, digits = index+1, digits+1
		}
	}
	if digits == 0 {
		// `.` and `+` on their own are not numbers, and neither is `0x10`'s `x`.
		return 0
	}
	// An exponent counts only if it is complete: `1e` is the number 1 followed by the
	// letter e, which is what makes `1e` + 0 equal 1.
	if index < len(text) && (text[index] == 'e' || text[index] == 'E') {
		exponent := index + 1
		if exponent < len(text) && (text[exponent] == '+' || text[exponent] == '-') {
			exponent++
		}
		exponentDigits := exponent
		for exponent < len(text) && text[exponent] >= '0' && text[exponent] <= '9' {
			exponent++
		}
		if exponent > exponentDigits {
			index = exponent
		}
	}
	return index
}
