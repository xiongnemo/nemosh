package runtime

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// The increment and decrement operators, split from arithmetic.go to stay under the
// 250-line ceiling. See arithmetic_command.go for what needed them.

// store writes a variable the way an assignment does, so `++` and `=` cannot end
// up disagreeing about what a write is.
//
// Through assignVar, which is what "the way an assignment does" has to mean. This
// wrote the map and marked the mutation, and said that was what the export
// tracking read; it was not. `export x=0; : $((x=5))` left a child seeing 0, and
// `readonly R; : $((R++))` changed R.
func (p *arithmeticParser) store(name string, value int64) error {
	if p.skipping > 0 {
		return nil
	}
	if err := checkArithmeticSubscript(name, p.depth); err != nil {
		return err
	}
	// A readonly name has raised the shell error that ends the expression. A write refused
	// otherwise -- an element before the front of its array -- has been said, and the
	// expression goes on with its value, as bash's does: `$((a[-9] = 5))` is 5. It ended
	// the script as a readonly variable, which it was not.
	if p.runtime.assignVar(name, strconv.FormatInt(value, 10)) != 0 && p.runtime.expansion.shellError {
		return errReadonlyTarget
	}
	return nil
}

// errReadonlyTarget is an arithmetic assignment assignVar refused. It has said so
// already and raised the shell error, so a caller reporting arithmetic errors
// passes over this one rather than saying it twice.
var errReadonlyTarget = errors.New("readonly variable")

// step applies a prefix `++` or `--` and answers with the new value.
func (p *arithmeticParser) step(operator string, _ bool) (int64, error) {
	name := p.peek()
	if !isArithmeticName(name) {
		return 0, fmt.Errorf("arithmetic syntax error: %s needs a variable, found %q", operator, name)
	}
	p.index++
	name, err := p.once(name)
	if err != nil {
		return 0, err
	}
	current, err := p.lookup(name)
	if err != nil {
		return 0, err
	}
	updated := stepped(current, operator)
	if err := p.store(name, updated); err != nil {
		return 0, err
	}
	return updated, nil
}

func stepped(value int64, operator string) int64 {
	if operator == "++" {
		return value + 1
	}
	return value - 1
}

func (p *arithmeticParser) peek() string {
	if p.index >= len(p.tokens) {
		return ""
	}
	return p.tokens[p.index]
}

func applyArithmetic(left int64, operator string, right int64) (int64, error) {
	switch operator {
	case "+":
		return left + right, nil
	case "-":
		return left - right, nil
	case "*":
		return left * right, nil
	case "/", "%":
		if right == 0 {
			return 0, fmt.Errorf("division by zero")
		}
		if operator == "/" {
			return left / right, nil
		}
		return left % right, nil
	// The count is taken modulo 64, as the machine does it and both references answer:
	// `5 << -1` is 5 << 63 and `1 << 64` is 1. Go's shift by 64 or more is 0, so every
	// out-of-range count, a negative one included, came out 0.
	case "<<":
		return left << (uint64(right) & 63), nil
	case ">>":
		return left >> (uint64(right) & 63), nil
	case "<":
		return boolValue(left < right), nil
	case "<=":
		return boolValue(left <= right), nil
	case ">":
		return boolValue(left > right), nil
	case ">=":
		return boolValue(left >= right), nil
	case "==":
		return boolValue(left == right), nil
	case "!=":
		return boolValue(left != right), nil
	case "&":
		return left & right, nil
	case "^":
		return left ^ right, nil
	case "|":
		return left | right, nil
	case "&&":
		return boolValue(left != 0 && right != 0), nil
	default:
		return boolValue(left != 0 || right != 0), nil
	}
}

func boolValue(condition bool) int64 {
	if condition {
		return 1
	}
	return 0
}

// power is `**`, and it is right-associative: measured, bash reads `2**3**2` as
// 2**(3**2), which is 512 rather than 64. Left-associative folding through the
// ordinary precedence table could not express that, so it has its own level between
// the binary operators and unary.
//
// It binds tighter than unary minus, which is the other thing measured: `-2**2` is 4
// in bash, not -4, because the minus applies to the result.
func (p *arithmeticParser) power() (int64, error) {
	left, err := p.unary()
	if err != nil {
		return 0, err
	}
	if p.peek() != "**" {
		return left, nil
	}
	p.index++
	right, err := p.power()
	if err != nil {
		return 0, err
	}
	value, err := integerPower(left, right)
	if err != nil && p.skipping > 0 {
		return 0, nil
	}
	return value, err
}

// apply is applyArithmetic, except in an arm that is not taken, where a division by zero is
// not an error because it is not done.
func (p *arithmeticParser) apply(left int64, operator string, right int64) (int64, error) {
	value, err := applyArithmetic(left, operator, right)
	if err != nil && p.skipping > 0 {
		return 0, nil
	}
	return value, err
}

// integerPower raises left to right by repeated multiplication, because these are
// integers and math.Pow would round.
func integerPower(left, right int64) (int64, error) {
	if right < 0 {
		// bash gives "exponent less than 0" and so does this: the answer is a
		// fraction, and there is nowhere to put one.
		return 0, fmt.Errorf("arithmetic: exponent less than 0")
	}
	result := int64(1)
	for count := int64(0); count < right; count++ {
		result *= left
	}
	return result, nil
}

// parseArithmeticInteger reads an integer constant as C and both references read one: 0x or
// 0X and hex digits, a leading 0 and octal ones, or decimal. The value wraps at 64 bits, as
// theirs do, so `9223372036854775808` is its own negative and `-9223372036854775808` is the
// least there is; each was a syntax error. strconv's reading also took Go's own forms, so
// `0b101`, `0o17` and `1_000` were numbers, and neither reference has any of them. A bare `0x`
// is 0, as in both.
func parseArithmeticInteger(token string) (int64, bool) {
	if token == "" {
		return 0, false
	}
	digits, base := token, uint64(10)
	switch {
	case len(token) >= 2 && token[0] == '0' && (token[1] == 'x' || token[1] == 'X'):
		digits, base = token[2:], 16
	case len(token) > 1 && token[0] == '0':
		digits, base = token[1:], 8
	}
	var value uint64
	for index := 0; index < len(digits); index++ {
		digit, ok := arithmeticDigit(digits[index], int64(base))
		if !ok {
			return 0, false
		}
		value = value*base + uint64(digit)
	}
	return int64(value), true
}

// parseArithmeticBase reads bash's `base#digits`: `2#101` is 5 and `16#ff` is 255.
//
// The `#` reported `unexpected "#"`, because the lexer had no such operator and the
// number parser had no such form. It is how a script reads a binary mask or a hex value
// without a leading 0x.
func parseArithmeticBase(token string) (int64, bool) {
	base, digits, found := strings.Cut(token, "#")
	if !found {
		return 0, false
	}
	radix, err := strconv.ParseInt(base, 10, 32)
	if err != nil || radix < 2 || radix > 64 || digits == "" {
		return 0, false
	}
	var value int64
	for index := 0; index < len(digits); index++ {
		digit, ok := arithmeticDigit(digits[index], radix)
		if !ok {
			return 0, false
		}
		value = value*radix + digit
	}
	return value, true
}

// arithmeticDigit is a digit's value in `base#digits`, and whether the base has it: 0-9, then
// a-z, then A-Z, then @ and _, which makes the 64 that bash allows. Up to base 36 a letter's
// case does not matter. The digits were strconv's, which stops at base 36, so anything above
// it -- `64#@`, `37#A` -- was a syntax error where both references read a number.
func arithmeticDigit(char byte, radix int64) (int64, bool) {
	var value int64
	switch {
	case char >= '0' && char <= '9':
		value = int64(char - '0')
	case char >= 'a' && char <= 'z':
		value = int64(char-'a') + 10
	case char >= 'A' && char <= 'Z':
		value = int64(char-'A') + 10
		if radix > 36 {
			value += 26
		}
	case char == '@':
		value = 62
	case char == '_':
		value = 63
	default:
		return 0, false
	}
	return value, value < radix
}
