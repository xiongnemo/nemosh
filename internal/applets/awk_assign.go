package applets

// Assignment, and the increment operators that are assignments wearing a shorter name.
//
// The one thing worth stating: **what is assigned keeps the kind it was given**. A string
// literal stays a string forever, so `x = "10"; x < 9` is true, while a field or a `-v`
// value stays a strnum and compares numerically. That falls out of storing the value
// rather than its text, and it is why `awkValue` carries a kind at all.
//
// A compound assignment is arithmetic, so `x .= ""` does not exist and `x += "3x"` adds 3.

func (in *awkInterp) evalAssign(node awkAssignExpr) (awkValue, error) {
	value, err := in.eval(node.value)
	if err != nil {
		return awkValue{}, err
	}
	if node.operator != "=" {
		current, err := in.loadLvalue(node.target)
		if err != nil {
			return awkValue{}, err
		}
		// `+=` and friends are arithmetic on both sides, whatever kinds they held.
		value, err = in.arithmetic(compoundOperator(node.operator), current.num(), value.num())
		if err != nil {
			return awkValue{}, err
		}
	}
	if err := in.storeLvalue(node.target, value); err != nil {
		return awkValue{}, err
	}
	return value, nil
}

// compoundOperator strips the `=` from `+=` and its relatives.
func compoundOperator(operator string) string {
	if operator == "**=" {
		return "^"
	}
	return operator[:len(operator)-1]
}

func (in *awkInterp) evalIncDec(node awkIncDecExpr) (awkValue, error) {
	current, err := in.loadLvalue(node.target)
	if err != nil {
		return awkValue{}, err
	}
	// Both operators are numeric whatever was there: `x="abc"; x++` leaves x at 1.
	before := current.num()
	step := 1.0
	if node.operator == "--" {
		step = -1
	}
	updated := awkNum(before + step)
	if err := in.storeLvalue(node.target, updated); err != nil {
		return awkValue{}, err
	}
	if node.prefix {
		return updated, nil
	}
	// The postfix form answers what was there **as a number**, which is why
	// `$0="a b c"; x=1; print $x++` prints 0 rather than `a`.
	return awkNum(before), nil
}

// loadLvalue reads whatever an assignable expression currently holds.
func (in *awkInterp) loadLvalue(target awkExpr) (awkValue, error) {
	switch node := target.(type) {
	case awkVarExpr:
		return in.getVar(node.name), nil
	case awkFieldExpr:
		return in.evalField(node)
	case awkIndexExpr:
		return in.evalIndex(node)
	}
	return awkValue{}, in.errorf("cannot read %s as a variable", awkReprintExpr(target))
}

// storeLvalue writes to a variable, a field or an array element.
func (in *awkInterp) storeLvalue(target awkExpr, value awkValue) error {
	switch node := target.(type) {
	case awkVarExpr:
		in.setVar(node.name, value)
		return nil
	case awkFieldExpr:
		subscript, err := in.eval(node.index)
		if err != nil {
			return err
		}
		index, err := in.fieldIndex(subscript)
		if err != nil {
			return err
		}
		in.setField(index, value.str(in.convfmt()))
		return nil
	case awkIndexExpr:
		key, err := in.subscript(node.index)
		if err != nil {
			return err
		}
		in.setArrayElement(node.name, key, value)
		return nil
	}
	return in.errorf("cannot assign to %s", awkReprintExpr(target))
}
