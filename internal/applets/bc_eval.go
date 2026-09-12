package applets

import (
	"bufio"
	"fmt"
	"io"
)

// bc's evaluator.
//
// Two rules decide almost everything a reader notices.
//
// **A top-level expression prints its value; an assignment does not.** `x + 1` prints,
// `x = 5` prints nothing, and `(x = 5)` prints 5 -- the parenthesis makes it an expression
// that happens to assign, which is why bc_ast.go keeps the group node.
//
// **`scale`, `ibase` and `obase` are ordinary variables** that the arithmetic reads. Setting
// `scale` changes every division that follows, which is what makes bc a calculator with a
// precision rather than a calculator with a type.

type bcInterp struct {
	variables map[string]bigDecimal
	arrays    map[string]map[int64]bigDecimal
	functions map[string]*bcFunction
	// returned is what the most recent `return` answered. It lives here rather than in
	// the flow signal because a signal is a plain value, and the returning statement is
	// several frames below the call that wants the answer.
	returned bigDecimal
	out      *bufio.Writer
	errors   io.Writer
	halted   bool
	depth    int
}

// bcFlow is what a statement asks the enclosing construct to do next.
type bcFlow uint8

const (
	bcFlowNone bcFlow = iota
	bcFlowBreak
	bcFlowContinue
	bcFlowReturn
	bcFlowHalt
)

func newBcInterp(stdout, stderr io.Writer) *bcInterp {
	interp := &bcInterp{
		variables: map[string]bigDecimal{},
		arrays:    map[string]map[int64]bigDecimal{},
		functions: map[string]*bcFunction{},
		out:       bufio.NewWriter(stdout),
		errors:    stderr,
	}
	// ibase and obase start at ten; scale at zero, which is why `10/3` is 3 until a
	// program says otherwise.
	interp.variables["ibase"] = decimalFromInt(10)
	interp.variables["obase"] = decimalFromInt(10)
	interp.variables["scale"] = decimalFromInt(0)
	interp.variables["last"] = decimalFromInt(0)
	return interp
}

func (in *bcInterp) setting(name string) int {
	value, ok := in.variables[name]
	if !ok {
		return 0
	}
	number := value.rescale(0).integer()
	if !number.IsInt64() {
		return 0
	}
	return int(number.Int64())
}

func (in *bcInterp) scale() int { return in.setting("scale") }
func (in *bcInterp) ibase() int { return in.setting("ibase") }
func (in *bcInterp) obase() int { return in.setting("obase") }

func (in *bcInterp) eval(expr bcExpr) (bigDecimal, error) {
	switch node := expr.(type) {
	case bcNumberExpr:
		// Converted here rather than at lex time, with whatever ibase is now in force.
		return parseDecimalDigits(node.digits, in.ibase())
	case bcNameExpr:
		return in.variables[node.name], nil
	case bcIndexExpr:
		return in.readArray(node)
	case bcGroupExpr:
		return in.eval(node.inner)
	case bcUnaryExpr:
		return in.evalUnary(node)
	case bcBinaryExpr:
		return in.evalBinary(node)
	case bcAssignExpr:
		return in.evalAssign(node)
	case bcIncDecExpr:
		return in.evalIncDec(node)
	case bcBuiltinExpr:
		return in.evalBuiltin(node)
	case bcCallExpr:
		return in.evalCall(node)
	case bcStringExpr:
		return bigDecimal{}, fmt.Errorf("a string is not a number")
	}
	return bigDecimal{}, fmt.Errorf("this expression is not supported")
}

func (in *bcInterp) readArray(node bcIndexExpr) (bigDecimal, error) {
	index, err := in.arrayIndex(node.index)
	if err != nil {
		return bigDecimal{}, err
	}
	return in.arrays[node.name][index], nil
}

// arrayIndex truncates to an integer, which is what a subscript is.
func (in *bcInterp) arrayIndex(expr bcExpr) (int64, error) {
	if expr == nil {
		return 0, fmt.Errorf("an array needs a subscript here")
	}
	value, err := in.eval(expr)
	if err != nil {
		return 0, err
	}
	number := value.rescale(0).integer()
	if !number.IsInt64() || number.Sign() < 0 {
		return 0, fmt.Errorf("array index out of range")
	}
	return number.Int64(), nil
}

func (in *bcInterp) evalUnary(node bcUnaryExpr) (bigDecimal, error) {
	operand, err := in.eval(node.operand)
	if err != nil {
		return bigDecimal{}, err
	}
	switch node.operator {
	case "-":
		return negateDecimal(operand), nil
	case "!":
		// A comparison answers 1 or 0, so its negation does too.
		return bcBoolean(operand.isZero()), nil
	}
	return bigDecimal{}, fmt.Errorf("unknown operator %s", node.operator)
}

func bcBoolean(value bool) bigDecimal {
	if value {
		return decimalFromInt(1)
	}
	return decimalFromInt(0)
}

func (in *bcInterp) evalBinary(node bcBinaryExpr) (bigDecimal, error) {
	// The logical pair short-circuits, so the right side is not evaluated when the left
	// settles it -- which matters because the right side may assign.
	switch node.operator {
	case "&&", "||":
		return in.evalLogical(node)
	}
	left, err := in.eval(node.left)
	if err != nil {
		return bigDecimal{}, err
	}
	right, err := in.eval(node.right)
	if err != nil {
		return bigDecimal{}, err
	}
	if bcRelational[node.operator] {
		return bcBoolean(bcComparisonHolds(node.operator, compareDecimal(left, right))), nil
	}
	return applyDecimal(node.operator[0], left, right, in.scale())
}

func (in *bcInterp) evalLogical(node bcBinaryExpr) (bigDecimal, error) {
	left, err := in.eval(node.left)
	if err != nil {
		return bigDecimal{}, err
	}
	if node.operator == "&&" && left.isZero() {
		return bcBoolean(false), nil
	}
	if node.operator == "||" && !left.isZero() {
		return bcBoolean(true), nil
	}
	right, err := in.eval(node.right)
	if err != nil {
		return bigDecimal{}, err
	}
	return bcBoolean(!right.isZero()), nil
}

func bcComparisonHolds(operator string, ordering int) bool {
	switch operator {
	case "==":
		return ordering == 0
	case "!=":
		return ordering != 0
	case "<":
		return ordering < 0
	case "<=":
		return ordering <= 0
	case ">":
		return ordering > 0
	case ">=":
		return ordering >= 0
	}
	return false
}
