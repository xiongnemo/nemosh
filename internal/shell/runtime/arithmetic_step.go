package runtime

import (
	"fmt"
	"strconv"
)

// The increment and decrement operators, split from arithmetic.go to stay under the
// 250-line ceiling. See arithmetic_command.go for what needed them.

// store writes a variable the way an assignment does, so `++` and `=` cannot end
// up disagreeing about what a write is -- both mark the mutation, which is what
// `set -a` and the export tracking read.
func (p *arithmeticParser) store(name string, value int64) error {
	p.runtime.vars[name] = strconv.FormatInt(value, 10)
	p.runtime.markVarMutation(name)
	return nil
}

// step applies a prefix `++` or `--` and answers with the new value.
func (p *arithmeticParser) step(operator string, _ bool) (int64, error) {
	name := p.peek()
	if !isVariableName(name) {
		return 0, fmt.Errorf("arithmetic syntax error: %s needs a variable, found %q", operator, name)
	}
	p.index++
	current, _ := p.lookup(name)
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
	case "<<":
		return left << uint64(right), nil
	case ">>":
		return left >> uint64(right), nil
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
	return integerPower(left, right)
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
