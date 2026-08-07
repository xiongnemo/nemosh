package runtime

import (
	"context"
	"errors"
	"fmt"
)

var (
	errPipelineMissingCommand = errors.New("missing command after pipeline")
	errPipelineEmptyCommand   = errors.New("empty command in pipeline")
)

type tokenListSegment struct {
	tokens   []shellToken
	operator tokenKind
}

func splitTokenList(tokens []shellToken) []tokenListSegment {
	segments := []tokenListSegment{{}}
	for _, token := range tokens {
		if token.kind == tokenAndIf || token.kind == tokenOrIf {
			segments = append(segments, tokenListSegment{operator: token.kind}, tokenListSegment{})
			continue
		}
		last := len(segments) - 1
		segments[last].tokens = append(segments[last].tokens, token)
	}
	return segments
}

func splitTokenPipeline(tokens []shellToken) ([][]shellToken, error) {
	commands := [][]shellToken{{}}
	for _, token := range tokens {
		if token.kind == tokenPipe {
			if len(commands[len(commands)-1]) == 0 {
				return nil, errPipelineEmptyCommand
			}
			commands = append(commands, []shellToken{})
			continue
		}
		commands[len(commands)-1] = append(commands[len(commands)-1], token)
	}
	if len(commands[len(commands)-1]) == 0 {
		return nil, errPipelineMissingCommand
	}
	return commands, nil
}

func (r Runtime) runTokenLine(ctx context.Context, tokens []shellToken, savedStatus int) lineResult {
	status := savedStatus
	operator := tokenWord
	for _, segment := range splitTokenList(tokens) {
		if len(segment.tokens) == 0 {
			operator = segment.operator
			continue
		}
		if operator == tokenAndIf && status != 0 || operator == tokenOrIf && status == 0 {
			continue
		}
		result := r.runTokenPipeline(ctx, segment.tokens, status)
		status = result.status
		if result.control != flowNone {
			return result
		}
	}
	return lineResult{status: status}
}

func (r Runtime) runTokenPipeline(ctx context.Context, tokens []shellToken, savedStatus int) lineResult {
	commands, err := splitTokenPipeline(tokens)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		return lineResult{status: 2}
	}
	if len(commands) == 1 {
		return r.runTokenCommand(ctx, commands[0], savedStatus)
	}
	pipeline, err := r.prepareTokenPipeline(ctx, commands)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		return lineResult{status: 1}
	}
	return r.executeTokenPipeline(ctx, pipeline, savedStatus)
}

func (r Runtime) runTokenCommand(ctx context.Context, tokens []shellToken, savedStatus int) lineResult {
	command, operations, err := parseRedirects(tokens)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		return lineResult{status: 1}
	}
	words := make([]word, len(command))
	for index, token := range command {
		words[index] = parseTypedWord(*token.parsed)
	}
	return r.runParsedWords(ctx, words, operations, savedStatus)
}

func (r Runtime) runParsedWords(ctx context.Context, command []word, operations []redirectOperation, savedStatus int) lineResult {
	var ok bool
	operations, ok = r.expandRedirectOperations(ctx, operations, savedStatus)
	if r.expansionFailed() {
		return unsetParameterResult()
	}
	if !ok {
		return lineResult{status: 1}
	}
	expanded := make([]shellToken, 0, len(command))
	for _, item := range command {
		values := r.expandWord(ctx, item, savedStatus)
		for _, value := range values {
			expanded = append(expanded, shellToken{kind: tokenWord, value: value})
		}
	}
	if r.expansionFailed() {
		return unsetParameterResult()
	}
	args := tokenValues(expanded)
	if len(args) == 0 {
		return lineResult{}
	}
	assignments, commandArgs := leadingAssignments(args)
	if len(assignments) > 0 && len(commandArgs) == 0 {
		return lineResult{status: r.assignVars(assignments)}
	}
	// Dispatch on the command, not on args[0]: with a leading assignment those
	// are different words, and reading the first one turned `V=x break` into a
	// lookup for a command named `break`. In a `while true` loop that never
	// ended.
	if result, handled := r.controlFlowBuiltin(ctx, commandArgs, assignments, operations, savedStatus); handled {
		return result
	}
	return lineResult{status: r.runCommandWithTokenAssignments(ctx, expanded, operations)}
}

// controlFlowBuiltin runs the builtins that answer with a control transfer
// rather than only a status. Each of them is a POSIX special builtin, so its
// leading assignments persist after it completes (2.9.1) and are applied here
// before the transfer leaves.
func (r Runtime) controlFlowBuiltin(ctx context.Context, args []string, assignments []assignment, operations []redirectOperation, savedStatus int) (lineResult, bool) {
	switch args[0] {
	case "exit", "exec", "return", "break", "continue":
	default:
		return lineResult{}, false
	}
	if status := r.assignVars(assignments); status != 0 {
		return lineResult{status: status}, true
	}
	switch args[0] {
	case "exit":
		return lineResult{status: exitStatus(args[1:], savedStatus), control: flowExit}, true
	case "exec":
		if len(args) == 1 {
			return lineResult{status: r.execRedirect(operations)}, true
		}
		return lineResult{status: r.execBuiltin(ctx, args[1:]), control: flowExec}, true
	case "return":
		status := exitStatus(args[1:], savedStatus)
		if r.sourceDepth == 0 && r.functionDepth == 0 {
			fmt.Fprintln(r.streams.Stderr, "return: not in a sourced script")
			return lineResult{status: status}, true
		}
		return lineResult{status: status, control: flowReturn}, true
	case "break":
		return lineResult{control: flowBreak}, true
	default:
		return lineResult{control: flowContinue}, true
	}
}

func (r Runtime) expandRedirectOperations(ctx context.Context, operations []redirectOperation, savedStatus int) ([]redirectOperation, bool) {
	for index, operation := range operations {
		if operation.kind != redirectInput && operation.kind != redirectOutput && operation.kind != redirectAppend {
			if operation.kind == redirectHeredoc && operation.expand {
				operations[index].body = r.expandHeredocBody(ctx, operation.body, savedStatus)
			}
			continue
		}
		fields := r.expandWord(ctx, operation.operand, savedStatus)
		if len(fields) != 1 {
			fmt.Fprintf(r.streams.Stderr, "nemosh: %s: %v\n", operation.path, errAmbiguousRedirect)
			return nil, false
		}
		operations[index].path = fields[0]
	}
	return operations, true
}

func (r Runtime) runCommandWithTokenAssignments(ctx context.Context, tokens []shellToken, operations []redirectOperation) int {
	args := tokenValues(tokens)
	assignments, commandArgs := leadingAssignments(args)
	if len(commandArgs) == 0 {
		return r.assignVars(assignments)
	}
	commandStart := len(args) - len(commandArgs)
	commandTokens := tokens[commandStart:]
	if len(assignments) == 0 {
		return r.runCommandWithRedirectOperations(ctx, commandTokens, operations)
	}
	if isSpecialBuiltin(commandArgs[0]) {
		if status := r.assignVars(assignments); status != 0 {
			return status
		}
		return r.runCommandWithRedirectOperations(ctx, commandTokens, operations)
	}
	commandRuntime := r.withLocalAssignments(assignments)
	if commandRuntime == nil {
		return 1
	}
	status := commandRuntime.runCommandWithRedirectOperations(ctx, commandTokens, operations)
	r.mergeBuiltinMutations(*commandRuntime)
	return status
}
