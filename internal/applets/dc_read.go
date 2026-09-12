package applets

import (
	"fmt"
	"strings"
)

// Reading dc's source: which command a character is, and the two things that are longer than
// one character -- a number and a `[bracketed string]`.
//
// Split from dc_ops.go for the 250-line ceiling AGENTS.md sets. The seam is reading against
// doing: everything here answers "what did the program say", and everything there answers
// "what does it mean".
//
// `step` reports how many *extra* characters it used, so a number, a string and a
// two-character register command all return through the same loop.

func (m *dcMachine) step(source string, index int) (int, error) {
	character := source[index]
	switch {
	// A-F start a number too, because a base above ten needs them as digits; dc's own
	// commands are all lower case or outside that range.
	case character >= '0' && character <= '9', character >= 'A' && character <= 'F',
		character == '.', character == '_':
		return m.readNumber(source, index)
	case character == '[':
		return m.readString(source, index)
	case strings.IndexByte("slSL<>=", character) >= 0:
		return m.registerCommand(source, index)
	}
	return 0, m.simpleCommand(character)
}

// readNumber reads a literal, where `_` is dc's minus sign -- the `-` is subtraction.
func (m *dcMachine) readNumber(source string, index int) (int, error) {
	start := index
	negative := source[index] == '_'
	if negative {
		index++
	}
	digits := index
	for index < len(source) && (isDcDigit(source[index]) || source[index] == '.') {
		index++
	}
	value, err := parseDecimalDigits(source[digits:index], m.inputBase)
	if err != nil {
		return index - start - 1, err
	}
	if negative {
		value = negateDecimal(value)
	}
	m.push(dcValue{number: value})
	return index - start - 1, nil
}

func isDcDigit(character byte) bool {
	return character >= '0' && character <= '9' || character >= 'A' && character <= 'F'
}

// readString reads `[...]`, which nests: `[a [b] c]` is one string.
func (m *dcMachine) readString(source string, index int) (int, error) {
	depth, start := 0, index
	for ; index < len(source); index++ {
		switch source[index] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				m.push(dcValue{text: source[start+1 : index], isText: true})
				return index - start, nil
			}
		}
	}
	return index - start - 1, fmt.Errorf("unterminated string")
}

// registerCommand handles the commands that name a register in the next character.
func (m *dcMachine) registerCommand(source string, index int) (int, error) {
	if index+1 >= len(source) {
		return 0, fmt.Errorf("%c needs a register", source[index])
	}
	name := source[index+1]
	switch source[index] {
	case 's':
		value, err := m.pop()
		if err != nil {
			return 1, err
		}
		// `s` replaces the register; `S` pushes onto it. The difference is what lets a
		// register be used as a stack.
		m.registers[name] = []dcValue{value}
	case 'S':
		value, err := m.pop()
		if err != nil {
			return 1, err
		}
		m.registers[name] = append(m.registers[name], value)
	case 'l':
		// An unset register reads as **zero** rather than failing, which is dc's rule and
		// what lets `la 1 + sa` count without the register being primed first.
		stack := m.registers[name]
		if len(stack) == 0 {
			m.push(dcValue{number: decimalFromInt(0)})
			return 1, nil
		}
		m.push(stack[len(stack)-1])
	case 'L':
		stack := m.registers[name]
		if len(stack) == 0 {
			m.push(dcValue{number: decimalFromInt(0)})
			return 1, nil
		}
		m.push(stack[len(stack)-1])
		m.registers[name] = stack[:len(stack)-1]
	default:
		return 1, m.conditional(source[index], name)
	}
	return 1, nil
}

// conditional is `<r`, `>r` and `=r`: execute the register's string when the comparison
// holds.
//
// **The comparison is of the top against the one below it**, which reads backwards from the
// order the two were written: `1 2 <a` does *not* run `a`, and `2 1 <a` does. Measured,
// because the documentation of every dc says it one way and reading the stack suggests the
// other, and getting it inverted is a loop that never ends rather than a visible error.
func (m *dcMachine) conditional(operator, name byte) error {
	top, err := m.popNumber()
	if err != nil {
		return err
	}
	below, err := m.popNumber()
	if err != nil {
		return err
	}
	ordering := compareDecimal(top, below)
	holds := false
	switch operator {
	case '<':
		holds = ordering < 0
	case '>':
		holds = ordering > 0
	case '=':
		holds = ordering == 0
	}
	if !holds {
		return nil
	}
	stack := m.registers[name]
	if len(stack) == 0 {
		return nil
	}
	return m.runString(stack[len(stack)-1])
}

// runString executes a string value, which is how `x` and the conditionals work.
func (m *dcMachine) runString(value dcValue) error {
	if !value.isText {
		// Executing a number pushes it back, which is dc's rule and keeps `1 x` harmless.
		m.push(value)
		return nil
	}
	if m.depth > 200 {
		return fmt.Errorf("executed strings nested more than 200 deep")
	}
	m.depth++
	defer func() { m.depth-- }()
	return m.execute(value.text)
}
