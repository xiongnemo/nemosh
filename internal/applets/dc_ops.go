package applets

import (
	"fmt"
	"math/big"
)

// What dc's one-character commands do, once dc_read.go has worked out which one it is.

func (m *dcMachine) simpleCommand(character byte) error {
	switch character {
	case '+', '-', '*', '/', '%', '^':
		return m.arithmetic(character)
	case 'v':
		return m.unary(character)
	case 'p':
		return m.print(true)
	case 'n':
		// GNU's `n`: print the top **without** a newline and pop it. busybox's `n` is its
		// `p` -- newline, no pop -- which leaves no way to print two numbers on one line.
		// Recorded in docs/support-matrix.md.
		if err := m.print(false); err != nil {
			return err
		}
		_, err := m.pop()
		return err
	case 'f':
		return m.printStack()
	case 'c':
		m.stack = nil
	case 'd':
		return m.duplicate()
	case 'r':
		return m.swap()
	case 'x':
		value, err := m.pop()
		if err != nil {
			return err
		}
		return m.runString(value)
	case 'k', 'i', 'o':
		return m.setParameter(character)
	case 'K':
		m.push(dcValue{number: decimalFromInt(int64(m.scale))})
	case 'I':
		m.push(dcValue{number: decimalFromInt(int64(m.inputBase))})
	case 'O':
		m.push(dcValue{number: decimalFromInt(int64(m.outBase))})
	case 'z':
		m.push(dcValue{number: decimalFromInt(int64(len(m.stack)))})
	case 'Z', 'X':
		return m.measure(character)
	case 'q':
		m.quit = true
	case 'Q':
		return m.quitLevels()
	case '!':
		// See dc.go: this package starts no processes.
		return fmt.Errorf("the shell escape ! is not supported")
	case '?':
		return fmt.Errorf("reading a line with ? is not supported")
	default:
		return fmt.Errorf("bad character '%c'", character)
	}
	return nil
}

func (m *dcMachine) arithmetic(operator byte) error {
	top, err := m.popNumber()
	if err != nil {
		return err
	}
	below, err := m.popNumber()
	if err != nil {
		// The first operand goes back, so a stack with one number on it still has it.
		m.push(dcValue{number: top})
		return err
	}
	result, err := applyDecimal(operator, below, top, m.scale)
	if err != nil {
		return err
	}
	m.push(dcValue{number: result})
	return nil
}

// applyDecimal is the arithmetic shared with bc, so the two cannot disagree about a scale.
func applyDecimal(operator byte, left, right bigDecimal, scale int) (bigDecimal, error) {
	switch operator {
	case '+':
		return addDecimal(left, right), nil
	case '-':
		return subDecimal(left, right), nil
	case '*':
		return mulDecimal(left, right, scale), nil
	case '/':
		return divDecimal(left, right, scale)
	case '%':
		return modDecimal(left, right, scale)
	case '^':
		if right.scale != 0 && !right.rescale(0).integer().IsInt64() {
			return bigDecimal{}, fmt.Errorf("exponent must be an integer")
		}
		return powDecimal(left, right.rescale(0).integer(), scale)
	}
	return bigDecimal{}, fmt.Errorf("unknown operator '%c'", operator)
}

func (m *dcMachine) unary(operator byte) error {
	value, err := m.popNumber()
	if err != nil {
		return err
	}
	result, err := sqrtDecimal(value, m.scale)
	if err != nil {
		m.push(dcValue{number: value})
		return err
	}
	m.push(dcValue{number: result})
	return nil
}

func (m *dcMachine) print(newline bool) error {
	if len(m.stack) == 0 {
		return fmt.Errorf("stack has too few elements")
	}
	value := m.stack[len(m.stack)-1]
	text := value.text
	if !value.isText {
		rendered, err := formatDecimal(value.number, m.outBase)
		if err != nil {
			return err
		}
		text = rendered
	}
	if newline {
		text += "\n"
	}
	_, err := m.out.WriteString(text)
	return err
}

// printStack is `f`, which prints the whole stack **top first** -- the order a reader would
// read it off, and the opposite of the order it was pushed.
func (m *dcMachine) printStack() error {
	for index := len(m.stack) - 1; index >= 0; index-- {
		value := m.stack[index]
		if value.isText {
			fmt.Fprintf(m.out, "%s\n", value.text)
			continue
		}
		text, err := formatDecimal(value.number, m.outBase)
		if err != nil {
			return err
		}
		fmt.Fprintf(m.out, "%s\n", text)
	}
	return nil
}

func (m *dcMachine) duplicate() error {
	if len(m.stack) == 0 {
		return fmt.Errorf("stack has too few elements")
	}
	m.push(m.stack[len(m.stack)-1])
	return nil
}

func (m *dcMachine) swap() error {
	if len(m.stack) < 2 {
		return fmt.Errorf("stack has too few elements")
	}
	last := len(m.stack) - 1
	m.stack[last], m.stack[last-1] = m.stack[last-1], m.stack[last]
	return nil
}

func (m *dcMachine) setParameter(which byte) error {
	value, err := m.popNumber()
	if err != nil {
		return err
	}
	number := value.rescale(0).integer()
	if !number.IsInt64() {
		return fmt.Errorf("value out of range")
	}
	setting := int(number.Int64())
	switch which {
	case 'k':
		if setting < 0 {
			return fmt.Errorf("scale must not be negative")
		}
		m.scale = setting
	case 'i':
		if setting < 2 || setting > 16 {
			return fmt.Errorf("input base must be between 2 and 16")
		}
		m.inputBase = setting
	case 'o':
		if setting < 2 || setting > 16 {
			return fmt.Errorf("output base must be between 2 and 16")
		}
		m.outBase = setting
	}
	return nil
}

// measure is `Z`, the count of digits, and `X`, the scale.
func (m *dcMachine) measure(which byte) error {
	value, err := m.pop()
	if err != nil {
		return err
	}
	if value.isText {
		if which == 'Z' {
			m.push(dcValue{number: decimalFromInt(int64(len(value.text)))})
			return nil
		}
		m.push(dcValue{number: decimalFromInt(0)})
		return nil
	}
	if which == 'Z' {
		m.push(dcValue{number: decimalFromInt(int64(digitCount(value.number)))})
		return nil
	}
	m.push(dcValue{number: decimalFromInt(int64(value.number.scale))})
	return nil
}

func (m *dcMachine) quitLevels() error {
	value, err := m.popNumber()
	if err != nil {
		return err
	}
	levels := value.rescale(0).integer()
	if levels.Sign() <= 0 || !levels.IsInt64() {
		return fmt.Errorf("Q needs a positive count")
	}
	if levels.Cmp(big.NewInt(int64(m.depth))) > 0 {
		// Leaving more levels than there are leaves the program, which is what the
		// references do rather than treating it as an error.
		m.quit = true
		return nil
	}
	m.quit = true
	return nil
}
