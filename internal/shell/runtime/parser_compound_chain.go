package runtime

import "strings"

// A compound joined to the one before it: `for ...; done | while read l; do ...; done`, `if
// ...; fi && if ...; fi`, `esac || while ...`. POSIX 2.9.3 makes each pipeline of an and-or
// list a command, so either side of an operator may be a compound, and both references run
// these. The span builder found a compound after an operator only when plain words stood
// before it, so in `fi && if` the words were a command called `fi`, and the first compound
// never closed: "missing fi".
//
// The line such a join is on closes one compound and opens the next. The span builder closes
// the first and marks the second afterCompound, and the program builder joins the two nodes
// the way the operator between them says.

// chainedCloser is the closer that the words before the operator opening another compound are,
// and the redirection or more of a list after it, as in `done < in | cat | while ...`.
func chainedCloser(prefix, operator string) (string, string, bool) {
	operator = strings.TrimSuffix(operator, negatedSuffix)
	if operator != "|" && operator != "|&" && operator != "&&" && operator != "||" && operator != "&" {
		return "", "", false
	}
	closer := strings.TrimSpace(prefix)
	if closer == "fi" || closer == "done" || closer == "esac" {
		return closer, "", true
	}
	return splitCompoundCloser(closer)
}

// closeChainedCompound closes the compound a line ends when the words before the operator that
// opens another are its closer; see chainedCloser. It answers whether it did.
func closeChainedCompound(stack *[]compoundFrame, spans *[]compoundSpan, prefix, operator string, index int) (bool, error) {
	closer, suffix, ok := chainedCloser(prefix, operator)
	if !ok {
		return false, nil
	}
	closed, err := closeCompound(*stack, closer, index)
	if err != nil {
		return false, err
	}
	*stack = (*stack)[:len(*stack)-1]
	closed.suffix = suffix
	*spans = append(*spans, closed)
	return true, nil
}

// compoundProgramNode is the node for the compound span begins, with what its lines add on
// either side and the compounds joined after it, and the line the last of them ends on.
func compoundProgramNode(lines []string, spans []compoundSpan, byStart map[int]int, span compoundSpan, budget *parseBudget, depth int) (programNode, int, error) {
	node, err := compoundWithAffixes(lines, spans, byStart, span, budget, depth)
	if err != nil {
		return nil, 0, err
	}
	for {
		next, ok := byStart[span.end]
		if !ok || !spans[next].afterCompound {
			break
		}
		span = spans[next]
		right, err := compoundWithAffixes(lines, spans, byStart, span, budget, depth)
		if err != nil {
			return nil, 0, err
		}
		if node, err = joinCompounds(node, span.prefixOperator, right, budget, depth); err != nil {
			return nil, 0, err
		}
	}
	if span.background {
		node = backgroundNode{value: node}
	}
	return node, span.end, nil
}

// compoundWithAffixes is the compound's node with the words after its closer on the closer's
// line, and the ones before its opener on the opener's.
func compoundWithAffixes(lines []string, spans []compoundSpan, byStart map[int]int, span compoundSpan, budget *parseBudget, depth int) (programNode, error) {
	node, err := parseTypedCompound(lines, spans, byStart, span, budget, depth)
	if err != nil {
		return nil, err
	}
	// The body in between moved the line on.
	budget.enterLine(span.end)
	if span.suffix != "" {
		// A compound with a redirection or a pipe after it is exactly a brace group holding
		// that compound: same scope, same redirects, same behaviour as a pipeline stage.
		// Reusing the group rather than adding a second thing that carries redirects is what
		// keeps the two from disagreeing -- `{ ...; } < file` already worked.
		if node, err = wrapCompoundWithSuffix(node, span.suffix, budget, depth); err != nil {
			return nil, err
		}
	}
	if span.prefixOperator != "" && !span.afterCompound {
		budget.enterLine(span.start)
		if node, err = wrapCompoundAfterOperator(node, span.prefix, span.prefixOperator, budget, depth); err != nil {
			return nil, err
		}
	}
	return node, nil
}

// joinCompounds joins two compounds the way the operator between them says. Either may be a
// list already, when its line made it one: `done | cat | while ...` pipes on from the cat.
func joinCompounds(left programNode, operator string, right programNode, budget *parseBudget, depth int) (programNode, error) {
	if base, negated := strings.CutSuffix(operator, negatedSuffix); negated {
		right, operator = negateCompound(right), base
	}
	if operator == "|&" {
		// `done |& while` is `done 2>&1 | while`, as for any command after it.
		redirected, err := wrapCompoundWithSuffix(left, "2>&1", budget, depth)
		if err != nil {
			return nil, err
		}
		left, operator = redirected, "|"
	}
	prior, following := compoundAsList(left), compoundAsList(right)
	last, first := &prior.items[len(prior.items)-1], following.items[0]
	switch operator {
	case "&":
		last.background = true
		prior.items = append(prior.items, following.items...)
		return listNode{value: prior}, nil
	case "|":
		// The right's first pipeline goes on as the left's last.
		stage := &last.value.pipelines[len(last.value.pipelines)-1]
		stage.commands = append(stage.commands, first.value.pipelines[0].commands...)
		last.value.pipelines = append(last.value.pipelines, first.value.pipelines[1:]...)
		last.value.operators = append(last.value.operators, first.value.operators...)
	default:
		kind := tokenAndIf
		if operator == "||" {
			kind = tokenOrIf
		}
		last.value.pipelines = append(last.value.pipelines, first.value.pipelines...)
		last.value.operators = append(append(last.value.operators, kind), first.value.operators...)
	}
	last.background = first.background
	prior.items = append(prior.items, following.items[1:]...)
	return listNode{value: prior}, nil
}
