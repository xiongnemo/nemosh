package applets

import (
	"math"
	"strconv"
	"strings"
)

// cNumber is the number C's strtod reads at the start of text, after blanks, and how far it read
// -- 0 for no number. busybox's -n reads a key with msvcrt's atof, which knows decimals and
// exponents and nothing else, so 0x10 and inf are 0; -g and -h read it with mingw's strtod, which
// knows hexadecimal, inf and nan as well, which is what extended asks for.
func cNumber(text string, extended bool) (float64, int) {
	index := 0
	for index < len(text) && isCSpace(text[index]) {
		index++
	}
	start := index
	if index < len(text) && (text[index] == '+' || text[index] == '-') {
		index++
	}
	negative := text[start:index] == "-"
	if extended {
		rest := strings.ToLower(text[index:])
		switch {
		case strings.HasPrefix(rest, "infinity"):
			return math.Inf(signOf(negative)), index + len("infinity")
		case strings.HasPrefix(rest, "inf"):
			return math.Inf(signOf(negative)), index + len("inf")
		case strings.HasPrefix(rest, "nan"):
			return math.NaN(), index + len("nan")
		case strings.HasPrefix(rest, "0x"):
			if value, end, ok := hexNumber(text, start, index+2); ok {
				return value, end
			}
		}
	}
	digits := 0
	for index < len(text) && isASCIIDigit(text[index]) {
		index, digits = index+1, digits+1
	}
	if index < len(text) && text[index] == '.' {
		for index++; index < len(text) && isASCIIDigit(text[index]); index++ {
			digits++
		}
	}
	if digits == 0 {
		return 0, 0
	}
	index = exponentEnd(text, index, 'e')
	// Out of range, ParseFloat answers ±Inf or 0, as strtod does.
	value, _ := strconv.ParseFloat(text[start:index], 64)
	return value, index
}

// hexNumber reads 0xH[.H][pEXP] from start, whose digits begin at digits.
func hexNumber(text string, start, digits int) (float64, int, bool) {
	index, count := digits, 0
	for index < len(text) && isHexDigit(text[index]) {
		index, count = index+1, count+1
	}
	if index < len(text) && text[index] == '.' {
		for index++; index < len(text) && isHexDigit(text[index]); index++ {
			count++
		}
	}
	if count == 0 {
		return 0, 0, false
	}
	mantissa := text[start:index]
	end := exponentEnd(text, index, 'p')
	exponent := text[index:end]
	if exponent == "" {
		exponent = "p0"
	}
	value, _ := strconv.ParseFloat(mantissa+exponent, 64)
	return value, end, true
}

// exponentEnd is past an exponent -- marker, a sign, digits -- at index, or index when there is
// none.
func exponentEnd(text string, index int, marker byte) int {
	if index >= len(text) || text[index]|0x20 != marker {
		return index
	}
	next := index + 1
	if next < len(text) && (text[next] == '+' || text[next] == '-') {
		next++
	}
	if next >= len(text) || !isASCIIDigit(text[next]) {
		return index
	}
	for next < len(text) && isASCIIDigit(text[next]) {
		next++
	}
	return next
}

func signOf(negative bool) int {
	if negative {
		return -1
	}
	return 1
}

func compareFloat(left, right float64) int {
	switch {
	case left > right:
		return 1
	case left < right:
		return -1
	}
	return 0
}

// compareGeneral is -g, and -h with human: what is no number sorts first, then nan, then the
// numbers, -inf to +inf; under -h a larger suffix sorts later whatever the numbers.
func compareGeneral(left, right string, human bool) int {
	leftValue, leftEnd := cNumber(left, true)
	rightValue, rightEnd := cNumber(right, true)
	switch {
	case leftEnd == 0 && rightEnd == 0:
		return 0
	case leftEnd == 0:
		return -1
	case rightEnd == 0:
		return 1
	case math.IsNaN(leftValue) && math.IsNaN(rightValue):
		return 0
	case math.IsNaN(leftValue):
		return -1
	case math.IsNaN(rightValue):
		return 1
	}
	if human {
		if leftScale, rightScale := scaleSuffix(left[leftEnd:]), scaleSuffix(right[rightEnd:]); leftScale != rightScale {
			return leftScale - rightScale
		}
	}
	return compareFloat(leftValue, rightValue)
}

// scaleSuffix is busybox's order of a number's suffix under -h: k or K, then M G T P E Z Y in
// capitals only, and -1 for none.
func scaleSuffix(tail string) int {
	if tail == "" {
		return -1
	}
	index := strings.IndexByte("kmgtpezy", tail[0]|0x20)
	if index < 0 || index != 0 && tail[0] >= 'a' {
		return -1
	}
	return index
}

var sortMonths = [...]string{"jan", "feb", "mar", "apr", "may", "jun", "jul", "aug", "sep", "oct", "nov", "dec"}

// monthOf is strptime's %b as busybox's -M reads a key: after blanks, a month's name or its first
// three letters, in any case.
func monthOf(key string) (int, bool) {
	text := strings.TrimLeft(key, " \t\n\v\f\r")
	if len(text) < 3 {
		return 0, false
	}
	prefix := strings.ToLower(text[:3])
	for index, month := range sortMonths {
		if prefix == month {
			return index, true
		}
	}
	return 0, false
}

// strverscmp is glibc's, which busybox's -V calls: a run of digits compares as a number, except
// that one with a leading zero is a fraction and sorts before the rest.
func strverscmp(left, right string) int {
	const (
		stateNormal, stateInteger, stateFraction, stateZeros = 0, 3, 6, 9
		compareBytes, compareLength                          = 2, 3
	)
	next := [...]int{
		stateNormal, stateInteger, stateZeros,
		stateNormal, stateInteger, stateInteger,
		stateNormal, stateFraction, stateFraction,
		stateNormal, stateFraction, stateZeros,
	}
	results := [...]int{
		compareBytes, compareBytes, compareBytes, compareBytes, compareLength, compareBytes, compareBytes, compareBytes, compareBytes,
		compareBytes, -1, -1, +1, compareLength, compareLength, +1, compareLength, compareLength,
		compareBytes, compareBytes, compareBytes, compareBytes, compareBytes, compareBytes, compareBytes, compareBytes, compareBytes,
		compareBytes, +1, +1, -1, compareBytes, compareBytes, -1, compareBytes, compareBytes,
	}
	at := func(text string, index int) int {
		if index < len(text) {
			return int(text[index])
		}
		return 0
	}
	class := func(c int) int {
		switch {
		case c == '0':
			return 2
		case '1' <= c && c <= '9':
			return 1
		}
		return 0
	}
	c1, c2, index := at(left, 0), at(right, 0), 1
	state := stateNormal + class(c1)
	for c1 == c2 {
		if c1 == 0 {
			return 0
		}
		state = next[state]
		c1, c2, index = at(left, index), at(right, index), index+1
		state += class(c1)
	}
	switch result := results[state*3+class(c2)]; result {
	case compareBytes:
		return c1 - c2
	case compareLength:
		for position := index; class(at(left, position)) != 0; position++ {
			if class(at(right, position)) == 0 {
				return 1
			}
		}
		if class(at(right, index+digitRun(left, index))) != 0 {
			return -1
		}
		return c1 - c2
	default:
		return result
	}
}

// digitRun is how many digits begin text at index.
func digitRun(text string, index int) int {
	count := 0
	for index+count < len(text) && isASCIIDigit(text[index+count]) {
		count++
	}
	return count
}
