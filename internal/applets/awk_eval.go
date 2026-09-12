package applets

import (
	"math"
	"strings"
)

// Evaluating an expression.
//
// The value model (awk_value.go) carries the hard part -- what compares as a number and
// what as a string -- so this is mostly dispatch. The three things worth saying:
//
//   - **A bare `/re/` means `$0 ~ /re/`.** As the right operand of `~` it is the pattern
//     itself, so the node survives to here rather than being rewritten by the parser.
//   - **`&&` and `||` short-circuit**, and awk's answer is 1 or 0 rather than the operand:
//     `print (2 && 3)` is 1.
//   - **Division by zero is an error**, not an infinity. Both references stop the program,
//     which is the honest answer for a language with no exception to catch.

func (in *awkInterp) eval(expr awkExpr) (awkValue, error) {
	switch node := expr.(type) {
	case awkNumberExpr:
		return awkNum(node.value), nil
	case awkStringExpr:
		return awkStr(node.value), nil
	case awkRegexExpr:
		// Standing alone, a regex is a match against the current record.
		matched, err := awkMatches(in.getRecord(), node.pattern)
		if err != nil {
			return awkValue{}, err
		}
		return awkBool(matched), nil
	case awkVarExpr:
		return in.getVar(node.name), nil
	case awkGroupExpr:
		return in.eval(node.inner)
	case awkFieldExpr:
		return in.evalField(node)
	case awkIndexExpr:
		return in.evalIndex(node)
	case awkUnaryExpr:
		return in.evalUnary(node)
	case awkBinaryExpr:
		return in.evalBinary(node)
	case awkConcatExpr:
		return in.evalConcat(node)
	case awkMatchExpr:
		return in.evalMatch(node)
	case awkInExpr:
		return in.evalIn(node)
	case awkTernaryExpr:
		return in.evalTernary(node)
	case awkAssignExpr:
		return in.evalAssign(node)
	case awkIncDecExpr:
		return in.evalIncDec(node)
	}
	return awkValue{}, in.errorf("this expression is not supported yet: %s", awkReprintExpr(expr))
}

func (in *awkInterp) evalField(node awkFieldExpr) (awkValue, error) {
	subscript, err := in.eval(node.index)
	if err != nil {
		return awkValue{}, err
	}
	index, err := in.fieldIndex(subscript)
	if err != nil {
		return awkValue{}, err
	}
	return in.getField(index), nil
}

func (in *awkInterp) evalIndex(node awkIndexExpr) (awkValue, error) {
	key, err := in.subscript(node.index)
	if err != nil {
		return awkValue{}, err
	}
	array := in.getArray(node.name)
	// Reading an absent element **creates** it, which is awk's rule and the thing that
	// makes `if (a[k])` grow the array. Both references do it, and a program that tests
	// membership without wanting that uses `in`.
	value, present := array[key]
	if !present {
		in.touchArrayKey(node.name, key)
		array[key] = awkValue{}
	}
	return value, nil
}

// subscript joins the parts of `a[i,j]` with SUBSEP.
func (in *awkInterp) subscript(index []awkExpr) (string, error) {
	if len(index) == 1 {
		value, err := in.eval(index[0])
		if err != nil {
			return "", err
		}
		return value.str(in.convfmt()), nil
	}
	parts := make([]string, 0, len(index))
	for _, item := range index {
		value, err := in.eval(item)
		if err != nil {
			return "", err
		}
		parts = append(parts, value.str(in.convfmt()))
	}
	return strings.Join(parts, in.vars["SUBSEP"].str(in.convfmt())), nil
}

func (in *awkInterp) evalUnary(node awkUnaryExpr) (awkValue, error) {
	operand, err := in.eval(node.operand)
	if err != nil {
		return awkValue{}, err
	}
	switch node.operator {
	case "-":
		return awkNum(-operand.num()), nil
	case "+":
		// Unary plus is not a no-op: it forces a number, so `+"3x"` is 3.
		return awkNum(operand.num()), nil
	case "!":
		return awkBool(!operand.boolean()), nil
	}
	return awkValue{}, in.errorf("unknown unary operator %s", node.operator)
}

func (in *awkInterp) evalBinary(node awkBinaryExpr) (awkValue, error) {
	// The logical pair short-circuits, so the right side is not evaluated at all when
	// the left settles it.
	switch node.operator {
	case "&&", "||":
		return in.evalLogical(node)
	}
	left, err := in.eval(node.left)
	if err != nil {
		return awkValue{}, err
	}
	right, err := in.eval(node.right)
	if err != nil {
		return awkValue{}, err
	}
	switch node.operator {
	case "<", "<=", ">", ">=", "==", "!=":
		return awkBool(awkCompareHolds(node.operator, compareAwkValues(left, right, in.convfmt()))), nil
	}
	return in.arithmetic(node.operator, left.num(), right.num())
}

func (in *awkInterp) evalLogical(node awkBinaryExpr) (awkValue, error) {
	left, err := in.eval(node.left)
	if err != nil {
		return awkValue{}, err
	}
	if node.operator == "&&" && !left.boolean() {
		return awkBool(false), nil
	}
	if node.operator == "||" && left.boolean() {
		return awkBool(true), nil
	}
	right, err := in.eval(node.right)
	if err != nil {
		return awkValue{}, err
	}
	// The result is 1 or 0, not the operand: `print (2 && 3)` is 1.
	return awkBool(right.boolean()), nil
}

// awkCompareHolds turns a three-way comparison into the answer an operator wants.
func awkCompareHolds(operator string, ordering int) bool {
	switch operator {
	case "<":
		return ordering < 0
	case "<=":
		return ordering <= 0
	case ">":
		return ordering > 0
	case ">=":
		return ordering >= 0
	case "==":
		return ordering == 0
	case "!=":
		return ordering != 0
	}
	return false
}

func (in *awkInterp) arithmetic(operator string, left, right float64) (awkValue, error) {
	switch operator {
	case "+":
		return awkNum(left + right), nil
	case "-":
		return awkNum(left - right), nil
	case "*":
		return awkNum(left * right), nil
	case "/":
		if right == 0 {
			return awkValue{}, in.errorf("division by zero")
		}
		return awkNum(left / right), nil
	case "%":
		if right == 0 {
			return awkValue{}, in.errorf("division by zero in %%")
		}
		// Truncated toward zero, which is C's fmod and what both references use --
		// `-5 % 3` is -2, not 1.
		return awkNum(math.Mod(left, right)), nil
	case "^":
		return awkNum(math.Pow(left, right)), nil
	}
	return awkValue{}, in.errorf("unknown operator %s", operator)
}

func (in *awkInterp) evalConcat(node awkConcatExpr) (awkValue, error) {
	var out strings.Builder
	for _, part := range node.parts {
		value, err := in.eval(part)
		if err != nil {
			return awkValue{}, err
		}
		out.WriteString(value.str(in.convfmt()))
	}
	// The result of a concatenation is a **string**, never a strnum, which is why
	// `(1 " " 2) < 3` compares as text.
	return awkStr(out.String()), nil
}

func (in *awkInterp) evalMatch(node awkMatchExpr) (awkValue, error) {
	left, err := in.eval(node.left)
	if err != nil {
		return awkValue{}, err
	}
	pattern, err := in.patternOf(node.right)
	if err != nil {
		return awkValue{}, err
	}
	matched, err := awkMatches(left.str(in.convfmt()), pattern)
	if err != nil {
		return awkValue{}, err
	}
	return awkBool(matched != node.negated), nil
}

// patternOf answers the pattern text of a regex operand.
//
// A literal `/re/` is the pattern; anything else is evaluated and its **string** is the
// pattern, which is what makes a dynamic regex work: `$0 ~ x` uses whatever `x` holds.
func (in *awkInterp) patternOf(expr awkExpr) (string, error) {
	if regex, ok := expr.(awkRegexExpr); ok {
		return regex.pattern, nil
	}
	value, err := in.eval(expr)
	if err != nil {
		return "", err
	}
	return value.str(in.convfmt()), nil
}

func (in *awkInterp) evalIn(node awkInExpr) (awkValue, error) {
	key, err := in.subscript(node.index)
	if err != nil {
		return awkValue{}, err
	}
	// `in` asks without creating, which is the whole reason to use it over `a[k]`.
	_, present := in.arrays[node.array][key]
	return awkBool(present), nil
}

func (in *awkInterp) evalTernary(node awkTernaryExpr) (awkValue, error) {
	condition, err := in.eval(node.condition)
	if err != nil {
		return awkValue{}, err
	}
	if condition.boolean() {
		return in.eval(node.yes)
	}
	return in.eval(node.no)
}
