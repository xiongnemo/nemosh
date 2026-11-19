package applets

import (
	"math"
	"strconv"
	"strings"
	"time"
)

// The time and bit built-ins: systime, strftime, mktime, and, or, xor, compl, lshift and rshift,
// which are busybox's (editors/awk.c:2798-2875) as well as gawk's.
//
// strftime is the one date and ts use (strftime.go), lenient as ts's is: an unknown conversion
// is left as written, as glibc leaves it. busybox-w32's goes through MSVCRT, which answers an
// empty string for %F, %T or anything it does not know and gives up after the year 3000 --
// `strftime("%Y", 1e12)` is 1970 there and 33658 here, which is gawk's answer. The last argument
// of strftime and of mktime is gawk's, and true means UTC.

// awkDefaultTimeFormat is busybox's; gawk's has %e where this has %d.
const awkDefaultTimeFormat = "%a %b %d %H:%M:%S %Z %Y"

func (in *awkInterp) evalTimeBuiltin(node awkBuiltinExpr) (awkValue, error) {
	switch node.name {
	case "systime":
		return awkNum(float64(time.Now().Unix())), nil
	case "mktime":
		spec, err := in.argText(node, 0)
		if err != nil {
			return awkValue{}, err
		}
		zone, err := in.awkZone(node, 1)
		if err != nil {
			return awkValue{}, err
		}
		return awkNum(awkMktime(spec, zone)), nil
	}
	format := awkDefaultTimeFormat
	if len(node.args) > 0 {
		text, err := in.argText(node, 0)
		if err != nil {
			return awkValue{}, err
		}
		format = text
	}
	when := time.Now()
	if len(node.args) > 1 {
		seconds, err := in.argNumber(node, 1)
		if err != nil {
			return awkValue{}, err
		}
		when = time.Unix(awkToInt64(seconds), 0)
	}
	zone, err := in.awkZone(node, 2)
	if err != nil {
		return awkValue{}, err
	}
	return awkStr(strftimeLike(when.In(zone), format)), nil
}

// awkZone is UTC when the argument at index is there and true, gawk's utc-flag, and the local
// zone otherwise.
func (in *awkInterp) awkZone(node awkBuiltinExpr, index int) (*time.Location, error) {
	if index >= len(node.args) {
		return time.Local, nil
	}
	value, err := in.argValue(node, index)
	if err != nil || !value.boolean() {
		return time.Local, err
	}
	return time.UTC, nil
}

// awkMktime reads "YYYY MM DD HH MM SS [DST]" as busybox's do_mktime does, with sscanf: fewer
// than six numbers is -1, and so is a month of 0 or a year from 0 to 1899 -- busybox compares
// them unsigned, so a negative one is carried as any field out of range is, into the next, as
// C's mktime carries it and time.Date does. DST is C's tm_isdst: 1 reads the clock as daylight
// time, 0 as standard time, and -1 or none lets the zone say.
func awkMktime(spec string, zone *time.Location) float64 {
	numbers := [7]int{6: -1}
	count := 0
	for _, field := range strings.Fields(spec) {
		if count == len(numbers) {
			break
		}
		// One sign and the digits after it, which is what %u reads; sscanf stops at what follows.
		end := 0
		if field[0] == '+' || field[0] == '-' {
			end++
		}
		for end < len(field) && field[end] >= '0' && field[end] <= '9' {
			end++
		}
		number, err := strconv.Atoi(field[:end])
		if err != nil {
			break
		}
		numbers[count] = number
		count++
		if end < len(field) {
			break
		}
	}
	if count < 6 || numbers[1] == 0 || (numbers[0] >= 0 && numbers[0] < 1900) {
		return -1
	}
	when := time.Date(numbers[0], time.Month(numbers[1]), numbers[2], numbers[3], numbers[4], numbers[5], 0, zone)
	if daylight := numbers[6]; daylight >= 0 && (daylight > 0) != when.IsDST() {
		delta := awkDaylightSaving(when)
		if daylight > 0 {
			delta = -delta
		}
		when = when.Add(delta)
	}
	return float64(when.Unix())
}

// awkDaylightSaving is how far the clock moves for daylight time in the year of when, which is
// an hour almost everywhere and half of one on Lord Howe Island.
func awkDaylightSaving(when time.Time) time.Duration {
	_, january := time.Date(when.Year(), time.January, 1, 0, 0, 0, 0, when.Location()).Zone()
	_, july := time.Date(when.Year(), time.July, 1, 0, 0, 0, 0, when.Location()).Zone()
	return time.Duration(max(january-july, july-january)) * time.Second
}

// evalBitBuiltin takes each operand as busybox's getvar_i_int does, truncated to an unsigned
// integer with a negative one in two's complement -- but at gawk's 53 bits, not busybox's C long.
// That long is 32 bits on Windows, which makes busybox-w32's `compl(0)` 4294967295 and wraps
// `lshift(1, 40)` to 256, and 64 on Linux, which a double cannot hold: compl(5) rounds to 2^64
// there and loses the bits it was asked for. 53 bits are what a double holds exactly, so every
// answer here is the integer it claims to be. A shift of 64 or more is 0, which C leaves
// undefined.
func (in *awkInterp) evalBitBuiltin(node awkBuiltinExpr) (awkValue, error) {
	var result uint64
	for index := range node.args {
		number, err := in.argNumber(node, index)
		if err != nil {
			return awkValue{}, err
		}
		operand := awkBits(number)
		switch {
		case index == 0:
			result = operand
		case node.name == "and":
			result &= operand
		case node.name == "or":
			result |= operand
		case node.name == "xor":
			result ^= operand
		case node.name == "lshift":
			result <<= operand
		case node.name == "rshift":
			result >>= operand
		}
	}
	if node.name == "compl" {
		result = ^result & awkBitMask
	}
	return awkNum(float64(result)), nil
}

// awkBitMask is the 53 bits of a double's significand, gawk's width for compl and for a negative
// operand.
const awkBitMask = 1<<53 - 1

// awkBits truncates, and takes a negative number in two's complement, clamped where Go would
// leave the conversion undefined.
func awkBits(number float64) uint64 {
	switch {
	case math.IsNaN(number):
		return 0
	case number < 0:
		return -awkBits(-number) & awkBitMask
	case number >= 2*twoToThe63:
		return math.MaxUint64
	}
	return uint64(number)
}
