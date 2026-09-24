package applets

import (
	"errors"
	"fmt"
)

// testEvaluator walks the POSIX 2.14 `test` grammar:
//
//	oexpr   : aexpr ( -o aexpr )*
//	aexpr   : nexpr ( -a nexpr )*
//	nexpr   : ! nexpr | primary
//	primary : ( oexpr ) | operand BINOP operand | UNOP operand | operand
//
// which is the same shape busybox builds out of oexpr/aexpr/nexpr/primary in
// coreutils/test.c. What was here before evaluated four forms -- a bare string,
// -n, -z, and = / != -- and answered false to everything else, so `test -f x`
// and `test 1 -lt 2` were not wrong so much as unimplemented while looking
// implemented.
type testEvaluator struct {
	args    []string
	index   int
	view    ProcessView
	streams [3]any
}

var errTestUnknownOperand = errors.New("unknown operand")

func (e *testEvaluator) evaluate() (bool, error) {
	if len(e.args) == 0 {
		return false, nil
	}
	result, err := e.orExpression()
	if err != nil {
		return false, err
	}
	if e.index < len(e.args) {
		return false, fmt.Errorf("%s: %w", e.args[e.index], errTestUnknownOperand)
	}
	return result, nil
}

func (e *testEvaluator) orExpression() (bool, error) {
	result, err := e.andExpression()
	if err != nil {
		return false, err
	}
	for e.peek() == "-o" {
		e.index++
		right, err := e.andExpression()
		if err != nil {
			return false, err
		}
		result = result || right
	}
	return result, nil
}

func (e *testEvaluator) andExpression() (bool, error) {
	result, err := e.notExpression()
	if err != nil {
		return false, err
	}
	for e.peek() == "-a" {
		e.index++
		right, err := e.notExpression()
		if err != nil {
			return false, err
		}
		result = result && right
	}
	return result, nil
}

func (e *testEvaluator) notExpression() (bool, error) {
	// A `!` with a binary operator after it is an operand being compared: `[ "!" = "!" ]`
	// is true in both references. POSIX settles the three-argument form on $2 first,
	// and this took the `!` for negation and then found `=` with nothing to its left. A `!`
	// with nothing after it is the one-argument form, a non-empty string: `test !` is true.
	if e.peek() != "!" || e.binaryFollows() || e.index+1 >= len(e.args) {
		return e.primary()
	}
	e.index++
	result, err := e.notExpression()
	return !result, err
}

func (e *testEvaluator) primary() (bool, error) {
	switch {
	case e.index >= len(e.args):
		return false, errors.New("argument expected")
	// Binary before a group for the same reason, `[ "(" = "(" ]`, and a lone `(` is the
	// one-argument form -- a non-empty string -- rather than a group with nothing in it.
	case e.binaryFollows():
		return e.binaryPrimary()
	case e.args[e.index] == "(" && e.index+1 < len(e.args):
		e.index++
		result, err := e.orExpression()
		if err != nil {
			return false, err
		}
		if e.peek() != ")" {
			return false, errors.New("closing paren expected")
		}
		e.index++
		return result, nil
	// A unary operator with nothing after it is the one-argument form -- a
	// non-empty string -- which is why this insists on having an operand.
	// `test -f` is true and `test ! -f` is false, both by POSIX 2.14's
	// argument-count rules.
	case isTestUnaryOperator(e.args[e.index]) && e.index+1 < len(e.args):
		operator, operand := e.args[e.index], e.args[e.index+1]
		e.index += 2
		return e.unaryPrimary(operator, operand)
	}
	operand := e.args[e.index]
	e.index++
	return operand != "", nil
}

// binaryFollows reports a binary operator after the current word with an operand after it.
// Checked before a group, a negation and a unary operator, because POSIX resolves the
// three-argument form on $2 first: `test -f = -f` compares two strings rather than asking
// whether a file named `=` exists, and `[ "(" = "(" ]` compares two parentheses.
func (e *testEvaluator) binaryFollows() bool {
	return e.index+2 < len(e.args) && isTestBinaryOperator(e.args[e.index+1])
}

func (e *testEvaluator) binaryPrimary() (bool, error) {
	if e.index+2 >= len(e.args) {
		return false, errors.New("argument expected")
	}
	left, operator, right := e.args[e.index], e.args[e.index+1], e.args[e.index+2]
	e.index += 3
	return e.applyBinary(left, operator, right)
}

func (e *testEvaluator) peek() string {
	if e.index >= len(e.args) {
		return ""
	}
	return e.args[e.index]
}

func isTestUnaryOperator(word string) bool {
	switch word {
	case "-b", "-c", "-d", "-e", "-f", "-g", "-h", "-k", "-L", "-p", "-r",
		"-s", "-S", "-t", "-u", "-w", "-x", "-z", "-n", "-O", "-G":
		return true
	}
	return false
}

func isTestBinaryOperator(word string) bool {
	switch word {
	case "=", "==", "!=", "<", ">", "-eq", "-ne", "-gt", "-ge", "-lt", "-le",
		"-nt", "-ot", "-ef":
		return true
	}
	return false
}
