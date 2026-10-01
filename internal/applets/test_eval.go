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
	if result, counted, err := e.countedForm(); counted {
		return result, err
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

// countedForm is POSIX 2.14's rules by the number of arguments, which busybox's test_main
// takes before its grammar, each once the leading `!`s are counted off: none is false, one
// is whether it is non-empty, and three with a binary operator in the middle are that
// comparison -- so `test '(' = ')'` compares two parentheses and `test ! '(' = ')'` negates
// that, where the grammar reads a group. Any other form goes to the grammar from the start,
// its `!`s and all.
func (e *testEvaluator) countedForm() (result, counted bool, err error) {
	args, negate := e.args, false
	for {
		switch {
		case len(args) == 0:
			return false, true, nil
		case len(args) == 1:
			return (args[0] != "") != negate, true, nil
		case len(args) == 3 && isTestBinaryOperator(args[1]):
			result, err := e.applyBinary(args[0], args[1], args[2])
			return result != negate, true, err
		case args[0] != "!":
			return false, false, nil
		}
		args, negate = args[1:], !negate
	}
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
	// A `!` with nothing after it is an operand, a non-empty string: `[ x -a ! ]` is true. Any
	// other negates what follows, a comparison too, as busybox's nexpr has it: `[ ! = ! -a x ]`
	// is an error in both references, the `=` negated and the second `!` left over. It was
	// compared, which is right only for three arguments, and countedForm takes those.
	if e.peek() != "!" || e.index+1 >= len(e.args) {
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
	// A `(` opens a group before a comparison is looked for, as busybox's primary has it:
	// `test 0 -eq 0 -a '(' = ')'` is true, its group holding `=`, a non-empty string. The
	// comparison came first, and compared the two parentheses; the three-argument `test '('
	// = ')'` does compare them, and countedForm takes it before the grammar.
	case e.args[e.index] == "(":
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
	case e.binaryFollows():
		return e.binaryPrimary()
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
// Checked before a unary operator, as busybox's primary does: `test -f = -f x` compares two
// strings rather than asking whether a file named `=` exists.
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

// isTestUnaryOperator includes -o, which is bash's option test where an operand follows it
// and test's OR between two expressions, where orExpression takes it first.
func isTestUnaryOperator(word string) bool {
	switch word {
	case "-b", "-c", "-d", "-e", "-f", "-g", "-h", "-k", "-L", "-p", "-r",
		"-s", "-S", "-t", "-u", "-w", "-x", "-z", "-n", "-O", "-G", "-v", "-o", "-R":
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
