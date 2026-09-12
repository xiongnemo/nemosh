package applets

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

// The built-in functions, and the one rule that runs through all of them.
//
// **They count runes, not bytes.** `length("héllo")` is 5 here, and `index`, `substr`,
// `match`, RSTART and RLENGTH all agree with it. The references do not settle this:
// busybox always counts bytes, while gawk counts bytes under `LC_ALL=C` and runes under a
// UTF-8 locale -- so the two only appeared to agree because the measurement ran in the C
// locale. That makes it a decision rather than a measurement, and it is taken the way the
// rest of this project takes it: `wc -m`, `rev`, `fold` and `sed`'s `y///` all count runes,
// for the reason docs/support-matrix.md:817 gives. Bytes would also let `substr` cut a
// UTF-8 sequence in half and emit invalid output, which runes cannot do.

// awkBuiltinArity is the fewest and the most arguments each built-in takes. A -1 means any
// number, which only `sprintf` wants.
var awkBuiltinArity = map[string][2]int{
	"length": {0, 1}, "substr": {2, 3}, "index": {2, 2}, "split": {2, 3},
	"sub": {2, 3}, "gsub": {2, 3}, "match": {2, 2}, "sprintf": {1, -1},
	"sin": {1, 1}, "cos": {1, 1}, "atan2": {2, 2}, "exp": {1, 1}, "log": {1, 1},
	"sqrt": {1, 1}, "int": {1, 1}, "rand": {0, 0}, "srand": {0, 1},
	"tolower": {1, 1}, "toupper": {1, 1},
	"system": {1, 1}, "close": {1, 1}, "fflush": {0, 1},
}

func (in *awkInterp) evalBuiltin(node awkBuiltinExpr) (awkValue, error) {
	if err := awkCheckArity(node); err != nil {
		return awkValue{}, err
	}
	switch node.name {
	case "length":
		return in.builtinLength(node)
	case "substr":
		return in.builtinSubstr(node)
	case "index":
		return in.builtinIndex(node)
	case "tolower", "toupper":
		return in.builtinCase(node)
	case "split":
		return in.builtinSplit(node)
	case "sub":
		return in.builtinSub(node, false)
	case "gsub":
		return in.builtinSub(node, true)
	case "match":
		return in.builtinMatch(node)
	case "sprintf":
		return in.builtinSprintf(node)
	case "sin", "cos", "atan2", "exp", "log", "sqrt", "int", "rand", "srand":
		return in.evalMathBuiltin(node)
	case "system":
		return in.builtinSystem(node)
	case "close":
		name, err := in.argText(node, 0)
		if err != nil {
			return awkValue{}, err
		}
		return awkNum(float64(in.closeStream(name))), nil
	case "fflush":
		name, err := in.argText(node, 0)
		if err != nil {
			return awkValue{}, err
		}
		return awkNum(float64(in.flushOutputs(name))), nil
	}
	return awkValue{}, in.errorf("%s is not supported yet", node.name)
}

// awkCheckArity refuses a call with the wrong number of arguments, in gawk's words.
func awkCheckArity(node awkBuiltinExpr) error {
	bounds, known := awkBuiltinArity[node.name]
	if !known {
		return nil
	}
	count := len(node.args)
	if count < bounds[0] || (bounds[1] >= 0 && count > bounds[1]) {
		return fmt.Errorf("%s was called with %d arguments", node.name, count)
	}
	return nil
}

// builtinLength is `length`, `length(x)` and `length(a)`.
//
// Written with no parentheses at all it means `length($0)`, which the parser already
// recorded as a call with no arguments. An **array** argument answers how many elements it
// holds: that is not in POSIX, but both references have it and scripts use it.
func (in *awkInterp) builtinLength(node awkBuiltinExpr) (awkValue, error) {
	if len(node.args) == 0 {
		return awkNum(float64(utf8.RuneCountInString(in.getRecord()))), nil
	}
	if name, ok := node.args[0].(awkVarExpr); ok {
		if array, present := in.lookupArray(name.name); present {
			return awkNum(float64(array.length())), nil
		}
	}
	text, err := in.argText(node, 0)
	if err != nil {
		return awkValue{}, err
	}
	return awkNum(float64(utf8.RuneCountInString(text))), nil
}

func (in *awkInterp) builtinSubstr(node awkBuiltinExpr) (awkValue, error) {
	text, err := in.argText(node, 0)
	if err != nil {
		return awkValue{}, err
	}
	start, err := in.argNumber(node, 1)
	if err != nil {
		return awkValue{}, err
	}
	length := 0.0
	if len(node.args) > 2 {
		if length, err = in.argNumber(node, 2); err != nil {
			return awkValue{}, err
		}
	}
	return awkStr(awkSubstr(text, start, len(node.args) > 2, length)), nil
}

// awkSubstr is the out-of-range rule, which is the fiddly part and was measured.
//
// **A start before the string is moved to 1 and the length is kept**, so
// `substr("hello", -2, 4)` is `hell` rather than `h`. That is not the same as taking the
// characters between positions `m` and `m+n-1`, which would answer `h`, and both references
// agree on `hell`. A non-integer argument is **truncated**, not rounded:
// `substr("hello", 1.5, 2.4)` is `he`.
//
// The comparisons are written negated so that a NaN argument -- `substr(s, "x")` -- falls
// to the safe side rather than reaching an int64 conversion Go leaves undefined.
func awkSubstr(text string, startValue float64, hasLength bool, lengthValue float64) string {
	runes := []rune(text)
	total := float64(len(runes))
	start := math.Trunc(startValue)
	length := total
	if hasLength {
		length = math.Trunc(lengthValue)
	}
	if !(start >= 1) {
		start = 1
	}
	if start > total {
		return ""
	}
	if !(length >= 0) {
		length = 0
	}
	// Clamped in float arithmetic, because `substr(s, 2, 1e20)` must answer the tail of
	// the string rather than overflow an int.
	if length > total-start+1 {
		length = total - start + 1
	}
	from := int(start) - 1
	return string(runes[from : from+int(length)])
}

// builtinIndex answers where one string first appears inside another, counting from 1.
//
// **An empty needle is found at position 1**, which is gawk and POSIX; busybox answers 0.
// It falls out of `strings.Index` returning 0 for an empty needle, so the rule is kept by
// not special-casing it.
func (in *awkInterp) builtinIndex(node awkBuiltinExpr) (awkValue, error) {
	haystack, err := in.argText(node, 0)
	if err != nil {
		return awkValue{}, err
	}
	needle, err := in.argText(node, 1)
	if err != nil {
		return awkValue{}, err
	}
	at := strings.Index(haystack, needle)
	if at < 0 {
		return awkNum(0), nil
	}
	return awkNum(float64(utf8.RuneCountInString(haystack[:at]) + 1)), nil
}

// builtinCase is `tolower` and `toupper`.
//
// Unicode-aware, so `toupper("héllo")` is `HÉLLO` where both references leave the `é`
// alone in a C locale. That is this file's rune rule again, and `strings.ToUpper` is what
// the rest of the applets already use.
func (in *awkInterp) builtinCase(node awkBuiltinExpr) (awkValue, error) {
	text, err := in.argText(node, 0)
	if err != nil {
		return awkValue{}, err
	}
	if node.name == "toupper" {
		return awkStr(strings.ToUpper(text)), nil
	}
	return awkStr(strings.ToLower(text)), nil
}

// argValue evaluates one argument, answering the uninitialised value when it is not there.
func (in *awkInterp) argValue(node awkBuiltinExpr, index int) (awkValue, error) {
	if index >= len(node.args) {
		return awkValue{}, nil
	}
	return in.eval(node.args[index])
}

func (in *awkInterp) argText(node awkBuiltinExpr, index int) (string, error) {
	value, err := in.argValue(node, index)
	if err != nil {
		return "", err
	}
	return value.str(in.convfmt()), nil
}

func (in *awkInterp) argNumber(node awkBuiltinExpr, index int) (float64, error) {
	value, err := in.argValue(node, index)
	if err != nil {
		return 0, err
	}
	return value.num(), nil
}

// awkToInt64 truncates toward zero, clamping rather than overflowing.
//
// Converting an out-of-range float64 to an integer is **undefined** in Go, so
// `printf "%d", 1e300` would print whatever the hardware left behind. The bound is the
// strict one for the reason awk_value.go:176 gives: `float64(math.MaxInt64)` rounds *up*
// to 2^63, so comparing with `<=` would admit 2^63 itself and wrap it negative.
func awkToInt64(number float64) int64 {
	switch {
	case math.IsNaN(number):
		return 0
	case number >= twoToThe63:
		return math.MaxInt64
	case number < math.MinInt64:
		return math.MinInt64
	}
	return int64(number)
}
