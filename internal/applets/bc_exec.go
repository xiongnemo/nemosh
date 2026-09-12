package applets

import "fmt"

// Running bc's statements.
//
// The one that carries a rule rather than an action is the bare expression: **it prints its
// value unless it is an assignment**, and it sets `last` either way. That is why a session
// can be a column of sums with no `print` in sight, and why `x = 5` is quiet.

func (in *bcInterp) execBody(body []bcStmt) (bcFlow, error) {
	for _, statement := range body {
		flow, err := in.exec(statement)
		if err != nil || flow != bcFlowNone {
			return flow, err
		}
	}
	return bcFlowNone, nil
}

func (in *bcInterp) exec(statement bcStmt) (bcFlow, error) {
	switch node := statement.(type) {
	case bcExprStmt:
		return bcFlowNone, in.execExpression(node)
	case bcStringStmt:
		_, err := in.out.WriteString(node.text)
		return bcFlowNone, err
	case bcPrintStmt:
		return bcFlowNone, in.execPrint(node)
	case bcBlockStmt:
		return in.execBody(node.body)
	case bcIfStmt:
		return in.execIf(node)
	case bcWhileStmt:
		return in.execWhile(node)
	case bcForStmt:
		return in.execFor(node)
	case bcBreakStmt:
		return bcFlowBreak, nil
	case bcContinueStmt:
		return bcFlowContinue, nil
	case bcReturnStmt:
		return in.execReturn(node)
	case bcHaltStmt:
		in.halted = true
		return bcFlowHalt, nil
	case bcDefineStmt:
		in.functions[node.function.name] = node.function
		return bcFlowNone, nil
	}
	return bcFlowNone, fmt.Errorf("this statement is not supported")
}

// execExpression evaluates and, unless it was an assignment, prints.
func (in *bcInterp) execExpression(node bcExprStmt) error {
	value, err := in.eval(node.expr)
	if err != nil {
		return err
	}
	in.variables["last"] = value
	if _, isAssignment := node.expr.(bcAssignExpr); isAssignment {
		return nil
	}
	return in.writeNumber(value)
}

func (in *bcInterp) writeNumber(value bigDecimal) error {
	text, err := formatDecimal(value, in.obase())
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(in.out, text)
	return err
}

// execPrint writes its items with **no separator and no trailing newline**, which is what
// makes `print x, "\n"` the way a program controls its own layout.
func (in *bcInterp) execPrint(node bcPrintStmt) error {
	for _, item := range node.items {
		if text, ok := item.(bcStringExpr); ok {
			if _, err := in.out.WriteString(text.text); err != nil {
				return err
			}
			continue
		}
		value, err := in.eval(item)
		if err != nil {
			return err
		}
		rendered, err := formatDecimal(value, in.obase())
		if err != nil {
			return err
		}
		if _, err := in.out.WriteString(rendered); err != nil {
			return err
		}
	}
	return nil
}

func (in *bcInterp) execIf(node bcIfStmt) (bcFlow, error) {
	condition, err := in.eval(node.condition)
	if err != nil {
		return bcFlowNone, err
	}
	if !condition.isZero() {
		return in.exec(node.then)
	}
	if node.otherwise != nil {
		return in.exec(node.otherwise)
	}
	return bcFlowNone, nil
}

func (in *bcInterp) execWhile(node bcWhileStmt) (bcFlow, error) {
	for {
		condition, err := in.eval(node.condition)
		if err != nil {
			return bcFlowNone, err
		}
		if condition.isZero() {
			return bcFlowNone, nil
		}
		flow, err := in.exec(node.body)
		if stop, out := bcLoopFlow(flow); stop {
			return out, err
		}
		if err != nil {
			return bcFlowNone, err
		}
	}
}

// execFor runs `for (init; condition; update) body`, where a missing condition is true.
func (in *bcInterp) execFor(node bcForStmt) (bcFlow, error) {
	if node.initialise != nil {
		if _, err := in.eval(node.initialise); err != nil {
			return bcFlowNone, err
		}
	}
	for {
		if node.condition != nil {
			condition, err := in.eval(node.condition)
			if err != nil {
				return bcFlowNone, err
			}
			if condition.isZero() {
				return bcFlowNone, nil
			}
		}
		flow, err := in.exec(node.body)
		if stop, out := bcLoopFlow(flow); stop {
			return out, err
		}
		if err != nil {
			return bcFlowNone, err
		}
		if node.update != nil {
			if _, err := in.eval(node.update); err != nil {
				return bcFlowNone, err
			}
		}
	}
}

// bcLoopFlow decides what a loop does with a signal from its body.
func bcLoopFlow(flow bcFlow) (stop bool, out bcFlow) {
	switch flow {
	case bcFlowNone, bcFlowContinue:
		return false, bcFlowNone
	case bcFlowBreak:
		return true, bcFlowNone
	}
	// return and halt travel further out.
	return true, flow
}

func (in *bcInterp) execReturn(node bcReturnStmt) (bcFlow, error) {
	if node.value == nil {
		// A bare `return` answers zero, which is what an unfinished function answers too.
		in.returned = decimalFromInt(0)
		return bcFlowReturn, nil
	}
	value, err := in.eval(node.value)
	if err != nil {
		return bcFlowNone, err
	}
	in.returned = value
	return bcFlowReturn, nil
}
