package applets

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// expr's operations, split from its parser for the file-size ceiling. The
// precedence grammar is in expr.go; what each operator *means* is here.

// compareExprValues compares numerically when both sides are numbers and as
// strings otherwise, which is POSIX's rule and the reason `expr 10 \> 9` is 1
// while `expr a10 \> a9` is 0.
func compareExprValues(operator, left, right string) string {
	leftNumber, leftIsNumber := exprNumber(left)
	rightNumber, rightIsNumber := exprNumber(right)
	var result bool
	if leftIsNumber && rightIsNumber {
		switch operator {
		case "=":
			result = leftNumber == rightNumber
		case "!=":
			result = leftNumber != rightNumber
		case "<":
			result = leftNumber < rightNumber
		case "<=":
			result = leftNumber <= rightNumber
		case ">":
			result = leftNumber > rightNumber
		case ">=":
			result = leftNumber >= rightNumber
		}
	} else {
		switch operator {
		case "=":
			result = left == right
		case "!=":
			result = left != right
		case "<":
			result = left < right
		case "<=":
			result = left <= right
		case ">":
			result = left > right
		case ">=":
			result = left >= right
		}
	}
	if result {
		return "1"
	}
	return "0"
}

// matchExpr implements `STRING : BRE`.
//
// Anchored at the start, which is what makes `expr abc : 'a*'` answer 1 rather
// than 3: it counts the characters `a*` matched from the beginning, and that is
// one. With a capture group it yields the group's text instead of a count, which
// is the form scripts use to pull a version number out of a string.
func matchExpr(value, pattern string) (string, error) {
	compiled, err := regexp.Compile("^(?:" + basicToGoRegexp(pattern) + ")")
	if err != nil {
		return "", fmt.Errorf("invalid regular expression: %s", pattern)
	}
	found := compiled.FindStringSubmatch(value)
	if found == nil {
		if strings.Contains(pattern, `\(`) {
			return "", nil
		}
		return "0", nil
	}
	if len(found) > 1 {
		return found[1], nil
	}
	return strconv.Itoa(len(found[0])), nil
}

// basicToGoRegexp translates the BRE spellings expr takes into Go's syntax:
// `\(` and `\)` group, and a bare `(` is a literal.
func basicToGoRegexp(pattern string) string {
	var out strings.Builder
	for index := 0; index < len(pattern); index++ {
		if pattern[index] == '\\' && index+1 < len(pattern) {
			switch pattern[index+1] {
			case '(', ')', '{', '}':
				out.WriteByte(pattern[index+1])
				index++
				continue
			}
			out.WriteByte(pattern[index])
			out.WriteByte(pattern[index+1])
			index++
			continue
		}
		switch pattern[index] {
		case '(', ')', '{', '}':
			out.WriteString(regexp.QuoteMeta(string(pattern[index])))
		default:
			out.WriteByte(pattern[index])
		}
	}
	return out.String()
}

func arithmetic(operator, left, right string) (string, error) {
	leftNumber, leftOk := exprNumber(left)
	rightNumber, rightOk := exprNumber(right)
	if !leftOk || !rightOk {
		return "", fmt.Errorf("non-integer argument")
	}
	switch operator {
	case "+":
		return strconv.FormatInt(leftNumber+rightNumber, 10), nil
	case "-":
		return strconv.FormatInt(leftNumber-rightNumber, 10), nil
	case "*":
		return strconv.FormatInt(leftNumber*rightNumber, 10), nil
	}
	if rightNumber == 0 {
		return "", fmt.Errorf("division by zero")
	}
	if operator == "/" {
		return strconv.FormatInt(leftNumber/rightNumber, 10), nil
	}
	return strconv.FormatInt(leftNumber%rightNumber, 10), nil
}

func exprNumber(value string) (int64, bool) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return parsed, err == nil
}

// isTruthy is expr's notion: neither empty nor the number zero.
func isTruthy(value string) bool {
	if value == "" {
		return false
	}
	if number, ok := exprNumber(value); ok {
		return number != 0
	}
	return true
}
