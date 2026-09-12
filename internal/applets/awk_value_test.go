package applets

import (
	"math"
	"testing"
)

// awk's value model. **Every expectation here was measured against gawk 5.4.1 and
// busybox-w32 1.38.0 before a line of the implementation was written**, and the two agree
// on all of it except the two divergences called out at the bottom.
//
// This is the part that cannot be retrofitted. Get strnum wrong and every comparison in
// every program is subtly wrong, in a way whose failing tests are exactly the ones nobody
// thinks to write -- so they are written first, here.

const testConvfmt = "%.6g"

// The comparison rule, which has three branches and all three matter.
func TestAwkValue_comparison(t *testing.T) {
	for _, test := range []struct {
		name  string
		left  awkValue
		right awkValue
		want  int
	}{
		// Two string literals compare as strings, so "10" sorts before "9".
		{name: `"10" < "9"`, left: awkStr("10"), right: awkStr("9"), want: -1},
		// Two numbers compare as numbers.
		{name: "10 vs 9", left: awkNum(10), right: awkNum(9), want: 1},

		// A string on either side forces a string comparison, with the other side
		// converted. This is the rule that makes the next three answers differ.
		{name: `10 == "10.0" is false`, left: awkNum(10), right: awkStr("10.0"), want: -1},
		{name: "10 == 10.0 is true", left: awkNum(10), right: awkNum(10.0), want: 0},
		{name: `strnum 10 == 10.0 is true`, left: awkStrnumOf("10"), right: awkNum(10.0), want: 0},
		{name: `strnum 10 == "10.0" is false`, left: awkStrnumOf("10"), right: awkStr("10.0"), want: -1},

		// Two strnums that both read as numbers compare numerically -- the case that
		// makes `$1 < $2` on `10 9` answer 0 rather than 1.
		{name: "strnum 10 vs strnum 9", left: awkStrnumOf("10"), right: awkStrnumOf("9"), want: 1},
		// One that does not read as a number is a string, so the pair compares as text.
		{name: "strnum 10 vs strnum abc", left: awkStrnumOf("10"), right: awkStrnumOf("abc"), want: -1},

		// Uninitialised is both "" and 0 at once.
		{name: "uninit == 0", left: awkValue{}, right: awkNum(0), want: 0},
		{name: `uninit == ""`, left: awkValue{}, right: awkStr(""), want: 0},
		{name: "uninit < 1", left: awkValue{}, right: awkNum(1), want: -1},

		// An assigned string literal stays a literal: `x = "10"; x < 9` is true.
		{name: `assigned "10" vs 9`, left: awkStr("10"), right: awkNum(9), want: -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := compareAwkValues(test.left, test.right, testConvfmt); got != test.want {
				t.Fatalf("compare = %d, want %d", got, test.want)
			}
			// And the mirror image, because a comparison that is not antisymmetric would
			// make sorting and `>` disagree with `<`.
			if got, want := compareAwkValues(test.right, test.left, testConvfmt), -test.want; got != want {
				t.Fatalf("reversed compare = %d, want %d", got, want)
			}
		})
	}
}

// What counts as "looking numeric" for a strnum. Blanks either side are allowed, a
// leading sign and an exponent are allowed, and hex and trailing junk are not.
func TestAwkValue_strnumClassification(t *testing.T) {
	for _, test := range []struct {
		text    string
		numeric bool
		number  float64
	}{
		{text: "10", numeric: true, number: 10},
		{text: " 7 ", numeric: true, number: 7},
		{text: "+5", numeric: true, number: 5},
		{text: "-5", numeric: true, number: -5},
		{text: "1e3", numeric: true, number: 1000},
		{text: "1E3", numeric: true, number: 1000},
		{text: "1e-3", numeric: true, number: 0.001},
		{text: ".5", numeric: true, number: 0.5},
		{text: "10.0", numeric: true, number: 10},
		{text: "\t42\n", numeric: true, number: 42},

		// Not numbers. Hex is the interesting one -- see the divergence test below.
		{text: "0x10", numeric: false, number: 0},
		{text: "12abc", numeric: false, number: 12},
		{text: "abc", numeric: false, number: 0},
		{text: "", numeric: false, number: 0},
		{text: " ", numeric: false, number: 0},
		{text: "+", numeric: false, number: 0},
		{text: ".", numeric: false, number: 0},
		// `1e` is the number 1 followed by a letter, because an exponent needs digits.
		{text: "1e", numeric: false, number: 1},
		{text: "1e+", numeric: false, number: 1},
		// Neither reference accepts these as numbers; a field reading `inf` is text.
		{text: "inf", numeric: false, number: 0},
		{text: "nan", numeric: false, number: 0},
		{text: "Infinity", numeric: false, number: 0},
	} {
		t.Run(test.text, func(t *testing.T) {
			value := awkStrnumOf(test.text)
			if value.looksNumeric != test.numeric {
				t.Errorf("looksNumeric = %v, want %v", value.looksNumeric, test.numeric)
			}
			// The numeric prefix is answered whether or not the whole string was one,
			// because `"12abc" + 0` is 12.
			if got := awkStr(test.text).num(); got != test.number {
				t.Errorf("num() = %v, want %v", got, test.number)
			}
		})
	}
}

// Truthiness follows the same rule: a *field* holding `0` is false, the string literal
// `"0"` is true. Both references agree, and the pair is the whole point.
func TestAwkValue_boolean(t *testing.T) {
	for _, test := range []struct {
		name  string
		value awkValue
		want  bool
	}{
		{name: `string "0" is true`, value: awkStr("0"), want: true},
		{name: `string "" is false`, value: awkStr(""), want: false},
		{name: `string "abc" is true`, value: awkStr("abc"), want: true},
		{name: "number 0 is false", value: awkNum(0), want: false},
		{name: "number 1 is true", value: awkNum(1), want: true},
		{name: "uninit is false", value: awkValue{}, want: false},
		{name: `strnum "0" is false`, value: awkStrnumOf("0"), want: false},
		{name: `strnum "0a" is true`, value: awkStrnumOf("0a"), want: true},
		{name: `strnum "" is false`, value: awkStrnumOf(""), want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := test.value.boolean(); got != test.want {
				t.Fatalf("boolean() = %v, want %v", got, test.want)
			}
		})
	}
}

// Number to string: an integral value prints as an integer whatever the format says.
func TestFormatAwkNumber(t *testing.T) {
	for _, test := range []struct {
		name   string
		value  float64
		format string
		want   string
	}{
		{name: "integral float", value: 3.0, format: testConvfmt, want: "3"},
		{name: "negative integral", value: -3.0, format: testConvfmt, want: "-3"},
		{name: "negative zero prints as zero", value: math.Copysign(0, -1), format: testConvfmt, want: "0"},
		{name: "a third", value: 1.0 / 3.0, format: testConvfmt, want: "0.333333"},
		{name: "CONVFMT is honoured", value: 1.0 / 3.0, format: "%.2g", want: "0.33"},
		{name: "one and a half", value: 1.5, format: testConvfmt, want: "1.5"},
		{name: "2^53 is exact", value: 9007199254740992, format: testConvfmt, want: "9007199254740992"},
		{name: "1e18 fits int64", value: 1e18, format: testConvfmt, want: "1000000000000000000"},
		// Past int64 the integer form stops, which is busybox's boundary. gawk prints
		// all twenty digits; see formatAwkNumber for why this follows busybox.
		{name: "1e19 does not fit", value: 1e19, format: testConvfmt, want: "1e+19"},
		{name: "negative past int64", value: -1e19, format: testConvfmt, want: "-1e+19"},
		// 2^63 is the boundary bug: float64(math.MaxInt64) rounds *up* to 2^63, so a
		// `<=` range check admitted it and int64() wrapped to a large negative. Both
		// references print it through %g.
		{name: "2^63 does not fit int64", value: 9223372036854775808.0, format: testConvfmt, want: "9.22337e+18"},
		{name: "min int64 does fit", value: -9223372036854775808.0, format: testConvfmt, want: "-9223372036854775808"},
		// Go renders these `+Inf` and `NaN`, which matches neither reference.
		{name: "infinity", value: math.Inf(1), format: testConvfmt, want: "+inf"},
		{name: "negative infinity", value: math.Inf(-1), format: testConvfmt, want: "-inf"},
		{name: "not a number", value: math.NaN(), format: testConvfmt, want: "nan"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := formatAwkNumber(test.value, test.format); got != test.want {
				t.Fatalf("formatAwkNumber(%v, %q) = %q, want %q", test.value, test.format, got, test.want)
			}
		})
	}
}

// The two places the references disagree, pinned with the choice and the reason, so that
// neither is quietly "fixed" later towards the other.
func TestAwkValue_referenceDivergences(t *testing.T) {
	// Hex. gawk says 0 and busybox says 16, because busybox hands the text to a strtod
	// that accepts hex. gawk is followed because busybox contradicts itself: it declines
	// to treat `0x10` as a strnum for comparison -- both references agree `$1 == 0` is
	// false for a field of `0x10` -- and then converts it as hex for arithmetic.
	if got := awkStr("0x10").num(); got != 0 {
		t.Errorf(`"0x10" + 0 = %v, want 0 (POSIX and gawk; busybox says 16)`, got)
	}
	if awkStrnumOf("0x10").looksNumeric {
		t.Error("0x10 classified as numeric; both references agree it is not")
	}

	// Past int64, busybox switches to %g and gawk prints every digit. busybox is the
	// primary reference, and a float64 has about seventeen significant digits, so
	// gawk's remaining three are artefacts rather than information.
	if got, want := formatAwkNumber(1e19, testConvfmt), "1e+19"; got != want {
		t.Errorf("1e19 = %q, want %q (busybox's rule, with C99's exponent width)", got, want)
	}
}
