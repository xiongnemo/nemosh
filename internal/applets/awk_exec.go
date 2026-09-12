package applets

import (
	"fmt"
	"strings"
)

// Executing statements.
//
// Control flow leaves a statement by a **signal** rather than by an error, because `break`,
// `next` and `exit` are ordinary things a program does and threading them through error
// returns would make every call site check whether a failure was really a failure.

// awkFlow is what a statement asks the enclosing construct to do next.
type awkFlow uint8

const (
	awkFlowNone awkFlow = iota
	awkFlowBreak
	awkFlowContinue
	// awkFlowNext abandons this record and reads the next one; awkFlowNextFile
	// abandons the whole file.
	awkFlowNext
	awkFlowNextFile
	// awkFlowExit leaves the record loop but still runs END, which is the one piece of
	// control flow in awk that is neither a loop nor a return.
	awkFlowExit
	awkFlowReturn
)

func (in *awkInterp) execBlock(body []awkStmt) (awkFlow, error) {
	for _, statement := range body {
		flow, err := in.exec(statement)
		if err != nil || flow != awkFlowNone {
			return flow, err
		}
	}
	return awkFlowNone, nil
}

func (in *awkInterp) exec(statement awkStmt) (awkFlow, error) {
	switch node := statement.(type) {
	case awkExprStmt:
		_, err := in.eval(node.expr)
		return awkFlowNone, err
	case awkPrintStmt:
		return awkFlowNone, in.execPrint(node)
	case awkPrintfStmt:
		return awkFlowNone, in.execPrintf(node)
	case awkBlockStmt:
		return in.execBlock(node.body)
	case awkIfStmt:
		return in.execIf(node)
	case awkWhileStmt:
		return in.execWhile(node)
	case awkDoStmt:
		return in.execDo(node)
	case awkForStmt:
		return in.execFor(node)
	case awkForInStmt:
		return in.execForIn(node)
	case awkDeleteStmt:
		return awkFlowNone, in.execDelete(node)
	case awkNextStmt:
		return awkFlowNext, nil
	case awkNextFileStmt:
		return awkFlowNextFile, nil
	case awkBreakStmt:
		return awkFlowBreak, nil
	case awkContinueStmt:
		return awkFlowContinue, nil
	case awkExitStmt:
		return in.execExit(node)
	}
	return awkFlowNone, in.errorf("this statement is not supported yet: %s", awkReprintStmt(statement))
}

// execPrint writes its arguments joined by OFS and terminated by ORS.
//
// No arguments means `$0`, which is what makes a bare `print` the commonest awk program
// there is.
func (in *awkInterp) execPrint(node awkPrintStmt) error {
	if node.redirect != nil {
		return in.errorf("print redirection is not supported yet")
	}
	// `print (a, b)` parses as one parenthesised list; the parser cannot tell that from
	// `print (expr)`, which must stay a group, so the list is unwrapped only here.
	args := awkUnwrapPrintList(node.args)
	if len(args) == 0 {
		return in.write(in.getRecord() + in.vars["ORS"].str(in.convfmt()))
	}
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		value, err := in.eval(arg)
		if err != nil {
			return err
		}
		// Output goes through OFMT rather than CONVFMT, which is the one place the two
		// differ.
		parts = append(parts, in.text(value))
	}
	separator := in.vars["OFS"].str(in.convfmt())
	return in.write(strings.Join(parts, separator) + in.vars["ORS"].str(in.convfmt()))
}

func (in *awkInterp) write(text string) error {
	_, err := fmt.Fprint(in.output, text)
	return err
}

func (in *awkInterp) execIf(node awkIfStmt) (awkFlow, error) {
	condition, err := in.eval(node.condition)
	if err != nil {
		return awkFlowNone, err
	}
	if condition.boolean() {
		return in.exec(node.then)
	}
	if node.otherwise != nil {
		return in.exec(node.otherwise)
	}
	return awkFlowNone, nil
}

func (in *awkInterp) execWhile(node awkWhileStmt) (awkFlow, error) {
	for {
		condition, err := in.eval(node.condition)
		if err != nil {
			return awkFlowNone, err
		}
		if !condition.boolean() {
			return awkFlowNone, nil
		}
		flow, err := in.exec(node.body)
		if stop, out := awkLoopFlow(flow); stop {
			return out, err
		}
		if err != nil {
			return awkFlowNone, err
		}
	}
}

func (in *awkInterp) execDo(node awkDoStmt) (awkFlow, error) {
	for {
		flow, err := in.exec(node.body)
		if stop, out := awkLoopFlow(flow); stop {
			return out, err
		}
		if err != nil {
			return awkFlowNone, err
		}
		condition, err := in.eval(node.condition)
		if err != nil {
			return awkFlowNone, err
		}
		if !condition.boolean() {
			return awkFlowNone, nil
		}
	}
}

func (in *awkInterp) execFor(node awkForStmt) (awkFlow, error) {
	if node.initialise != nil {
		if _, err := in.exec(node.initialise); err != nil {
			return awkFlowNone, err
		}
	}
	for {
		// An absent condition is true forever, which is what `for (;;)` means.
		if node.condition != nil {
			condition, err := in.eval(node.condition)
			if err != nil {
				return awkFlowNone, err
			}
			if !condition.boolean() {
				return awkFlowNone, nil
			}
		}
		flow, err := in.exec(node.body)
		if stop, out := awkLoopFlow(flow); stop {
			return out, err
		}
		if err != nil {
			return awkFlowNone, err
		}
		if node.step != nil {
			if _, err := in.exec(node.step); err != nil {
				return awkFlowNone, err
			}
		}
	}
}

// execForIn walks an array's keys.
//
// **Insertion order**, which POSIX leaves unspecified and both references answer
// differently. A stable order is chosen for the reason array_associative.go gives for the
// shell's arrays: an answer that changes between two runs of the same script is not
// something anyone can build on. Recorded in the support matrix as a deliberate choice.
func (in *awkInterp) execForIn(node awkForInStmt) (awkFlow, error) {
	for _, key := range in.arrayKeys(node.array) {
		// A key is a strnum, so `for (k in a)` over numeric subscripts compares
		// numerically inside the body.
		in.setVar(node.name, awkStrnumOf(key))
		flow, err := in.exec(node.body)
		if stop, out := awkLoopFlow(flow); stop {
			return out, err
		}
		if err != nil {
			return awkFlowNone, err
		}
	}
	return awkFlowNone, nil
}

func (in *awkInterp) execDelete(node awkDeleteStmt) error {
	if len(node.index) == 0 {
		// `delete a` empties the array but leaves the name an array, so a later
		// `a[k]=v` still works.
		in.arrays[node.name] = map[string]awkValue{}
		in.arrayOrders[node.name] = nil
		return nil
	}
	key, err := in.subscript(node.index)
	if err != nil {
		return err
	}
	in.deleteArrayKey(node.name, key)
	return nil
}

func (in *awkInterp) execExit(node awkExitStmt) (awkFlow, error) {
	if node.status != nil {
		status, err := in.eval(node.status)
		if err != nil {
			return awkFlowNone, err
		}
		in.exitStatus = int(status.num())
	}
	in.exiting = true
	return awkFlowExit, nil
}

// awkLoopFlow decides what a loop does with a signal from its body, and answers whether
// the loop should stop and what it should pass outward.
//
// `break` and `continue` are consumed here; `next`, `nextfile`, `exit` and `return` pass
// through, because they belong to something further out.
func awkLoopFlow(flow awkFlow) (stop bool, out awkFlow) {
	switch flow {
	case awkFlowNone, awkFlowContinue:
		return false, awkFlowNone
	case awkFlowBreak:
		return true, awkFlowNone
	}
	return true, flow
}
