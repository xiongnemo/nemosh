package runtime

import "context"

func (r Runtime) executeTypedIf(ctx context.Context, node ifNode, savedStatus int) lineResult {
	// A failing condition is the question, not an error, so `set -e` does not
	// act on it (POSIX 2.9.1).
	condition := r.suppressingErrExit().executeTypedList(ctx, node.condition, savedStatus)
	if condition.control != flowNone {
		return condition
	}
	body := node.thenBody
	if condition.status != 0 {
		body = node.elseBody
	}
	if len(body) == 0 {
		return lineResult{status: 0}
	}
	status, control := r.executeProgram(ctx, body, condition.status)
	return lineResult{status: status, control: control}
}

func (r Runtime) executeTypedLoop(ctx context.Context, node loopNode, savedStatus int) lineResult {
	if node.kind == loopFor {
		return r.executeTypedFor(ctx, node, savedStatus)
	}
	status := 0
	for {
		condition := r.suppressingErrExit().executeTypedList(ctx, node.condition, savedStatus)
		if ctx.Err() != nil {
			return lineResult{status: contextStatus(ctx)}
		}
		if condition.control != flowNone {
			return condition
		}
		if node.kind == loopWhile && condition.status != 0 || node.kind == loopUntil && condition.status == 0 {
			return lineResult{status: status}
		}
		bodyStatus, control := r.executeProgram(ctx, node.body, condition.status)
		status, savedStatus = bodyStatus, bodyStatus
		if ctx.Err() != nil {
			return lineResult{status: contextStatus(ctx)}
		}
		switch control {
		case flowNone:
		case flowContinue:
			status, savedStatus = 0, 0
		case flowBreak:
			return lineResult{status: 0}
		default:
			return lineResult{status: status, control: control}
		}
	}
}

func (r Runtime) executeTypedFor(ctx context.Context, node loopNode, savedStatus int) lineResult {
	status := 0
	for _, item := range node.values {
		if ctx.Err() != nil {
			return lineResult{status: contextStatus(ctx)}
		}
		values := r.expandWord(ctx, item, savedStatus)
		if r.expansionFailed() {
			return unsetParameterResult()
		}
	iteration:
		for _, value := range values {
			r.vars[node.name] = value
			bodyStatus, control := r.executeProgram(ctx, node.body, savedStatus)
			status, savedStatus = bodyStatus, bodyStatus
			if ctx.Err() != nil {
				return lineResult{status: contextStatus(ctx)}
			}
			switch control {
			case flowNone:
			case flowContinue:
				status, savedStatus = 0, 0
				continue iteration
			case flowBreak:
				return lineResult{status: 0}
			default:
				return lineResult{status: status, control: control}
			}
		}
	}
	return lineResult{status: status}
}

func (r Runtime) executeTypedCase(ctx context.Context, node caseNode, savedStatus int) lineResult {
	values := r.expandWord(ctx, node.word, savedStatus)
	if r.expansionFailed() {
		return unsetParameterResult()
	}
	value := ""
	if len(values) > 0 {
		value = values[0]
	}
	for _, arm := range node.arms {
		if !r.caseArmMatches(ctx, arm, value, savedStatus) {
			continue
		}
		status, control := r.executeProgram(ctx, arm.body, savedStatus)
		return lineResult{status: status, control: control}
	}
	return lineResult{status: 0}
}

// An arm matches when any one of its `|`-separated patterns does. The pattern
// is matched rather than compared, so `*`, `?`, and bracket expressions mean
// what POSIX 2.13.1 says they mean and not just the two literals -- an exact
// string and a lone `*` -- that used to be recognised here.
func (r Runtime) caseArmMatches(ctx context.Context, arm caseArmNode, value string, savedStatus int) bool {
	for _, candidate := range arm.patterns {
		expanded := r.expandWord(ctx, candidate, savedStatus)
		pattern := ""
		if len(expanded) > 0 {
			pattern = expanded[0]
		}
		if matchShellPattern(pattern, value) {
			return true
		}
	}
	return false
}
