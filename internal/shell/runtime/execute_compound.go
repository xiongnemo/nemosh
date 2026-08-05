package runtime

import "context"

func (r Runtime) executeTypedIf(ctx context.Context, node ifNode, savedStatus int) lineResult {
	condition := r.executeTypedList(ctx, node.condition, savedStatus)
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
		condition := r.executeTypedList(ctx, node.condition, savedStatus)
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
	value := ""
	if len(values) > 0 {
		value = values[0]
	}
	for _, arm := range node.arms {
		patterns := r.expandWord(ctx, arm.pattern, savedStatus)
		pattern := ""
		if len(patterns) > 0 {
			pattern = patterns[0]
		}
		if pattern != value && pattern != "*" {
			continue
		}
		status, control := r.executeProgram(ctx, arm.body, savedStatus)
		return lineResult{status: status, control: control}
	}
	return lineResult{status: 0}
}
