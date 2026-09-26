package runtime

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
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
		// A one-element $PIPESTATUS, as bash gives, so a read after a plain command
		// does not find the previous pipeline's leftovers.
		result := r.runTokenCommand(ctx, commands[0], savedStatus)
		r.recordPipeStatus(result.status)
		return result
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
	// `<(command)` left a temporary file behind, and this is the point that knows the
	// consumer has finished reading it. Deferred from here rather than from the expansion
	// so `cat <(echo hi)` still has a file to open; a snapshot carries its own expansion
	// state, so a background job or a subshell removes its own and not another's.
	defer r.cleanUpProcessSubstitutions()
	var ok bool
	operations, ok = r.expandRedirectOperations(ctx, operations, savedStatus)
	if r.shellErrorRaised() {
		return shellErrorResult()
	}
	if !ok {
		return lineResult{status: 1}
	}
	// `[[ ]]` is handled here, before expansion, because that is the whole of
	// what makes it different: inside it a word is not split and not globbed, and
	// whether the right-hand side was quoted still matters. See double_bracket.go.
	if isDoubleBracket(command) {
		return r.runDoubleBracket(ctx, command, savedStatus)
	}
	// Array assignments are also settled before expansion, and for the same
	// reason: `a=(one "two words" three)` is three elements, and after expansion
	// the quotes are gone. See array_assign.go.
	if remaining, applied := r.applyArrayAssignments(ctx, command, savedStatus); applied {
		if r.shellErrorRaised() {
			return shellErrorResult()
		}
		if len(remaining) == 0 {
			return lineResult{}
		}
		command = remaining
	}
	expanded := make([]shellToken, 0, len(command))
	// Where this command's own substitutions begin, so an assignment-only command can exit
	// with the status of the last one *it* performed rather than one from an earlier line.
	mark := r.expansion.substitutionMark()
	// Leading assignments are expanded unsplit. Recognised on the word rather than
	// on its expansion, which is the only place the distinction still exists: see
	// assignment_expand.go for what `d=$(date)` did without this.
	leading, declaration := true, false
	for _, item := range command {
		var values []string
		if (leading || declaration) && isAssignmentWord(item) {
			values = r.expandAssignmentWord(ctx, item, savedStatus)
		} else {
			declaration = declaration || leading && isDeclarationUtility(item)
			leading = false
			values = r.expandCommandWord(ctx, item, savedStatus)
		}
		for _, value := range values {
			expanded = append(expanded, shellToken{kind: tokenWord, value: value})
		}
	}
	if r.shellErrorRaised() {
		return shellErrorResult()
	}
	args := tokenValues(expanded)
	if len(args) == 0 {
		status, _ := r.expansion.substitutionStatusSince(mark)
		return r.redirectionsOnly(operations, lineResult{status: status})
	}
	assignments, commandArgs := leadingAssignments(args)
	if len(assignments) > 0 && len(commandArgs) == 0 {
		r.traceCommand(ctx, args, savedStatus)
		if failed := r.redirectionsOnly(operations, lineResult{}); failed.status != 0 {
			return failed
		}
		status := r.assignmentStatus(assignments, mark)
		r.vars["_"] = ""
		return r.abortOnShellError(lineResult{status: status})
	}
	// Alias substitution goes here rather than during tokenization, because
	// parsing completes before anything runs; see substituteAliases. A quoted
	// command name is not an alias, so the word as it was written decides --
	// and with a leading assignment in front, command[0] is that assignment
	// rather than the command, so there is no word here to judge.
	if len(commandArgs) > 0 && len(assignments) == 0 && len(command) > 0 && isUnquotedLiteralWord(command[0]) {
		substituted := r.substituteAliases(commandArgs)
		if !slices.Equal(substituted, commandArgs) {
			// The leading assignments keep their place in front; only the
			// command and its arguments are replaced.
			expanded = replaceCommandTokens(expanded, len(args)-len(commandArgs), substituted)
			commandArgs = substituted
		}
	}
	r.traceCommand(ctx, args, savedStatus)
	result := r.dispatchCommand(ctx, commandArgs, assignments, expanded, operations, savedStatus)
	// `$_` is the last argument of the command that just finished, or its name when
	// it had none, and it is set after the command -- so a function's own `$_` is its
	// last argument once it returns, whatever its body did. It was whatever the
	// environment brought in, for the whole session. bash's rule; busybox has no `$_`.
	r.vars["_"] = commandArgs[len(commandArgs)-1]
	return r.abortOnShellError(result)
}

// dispatchCommand runs a simple command once it is expanded. On the command, not on
// args[0]: with a leading assignment those are different words, and reading the first
// one turned `V=x break` into a lookup for a command named `break`. In a `while true`
// loop that never ended.
func (r Runtime) dispatchCommand(ctx context.Context, commandArgs []string, assignments []assignment, expanded []shellToken, operations []redirectOperation, savedStatus int) lineResult {
	if result, handled := r.controlFlowBuiltin(ctx, commandArgs, assignments, operations, savedStatus); handled {
		return result
	}
	if result, handled := r.functionCommand(ctx, commandArgs, assignments, operations); handled {
		return result
	}
	return lineResult{status: r.runCommandWithTokenAssignments(ctx, expanded, operations)}
}

// assignmentStatus is what a command consisting only of assignments exits with.
//
// POSIX: "If there is no command name, but the command contains a command substitution, the command
// shall complete with the exit status of the last command substitution performed." So `out=$(false)`
// is a failure, and that is what makes the commonest error check in shell work:
//
//	out=$(command) || die "command failed"
//
// This returned the assignment's own status, which is zero unless the *variable* refused the write,
// so that line never fired. bash, dash, ash and busybox-w32 all report the substitution's status;
// measured against bash while fixing it.
//
// A failed assignment still wins: `readonly x; x=$(true)` is an error about x, not a success.
// redirectionsOnly performs the redirections of a command with no command name, and
// answers result when they all succeed. POSIX 2.9.1 performs them, as both references do:
// `> file` creates or empties the file, `< missing` fails with status 1. They were dropped,
// so `> file` did nothing. When assignments come with them, a failure here leaves the
// assignments undone, as busybox-w32 has it; bash assigns first.
func (r Runtime) redirectionsOnly(operations []redirectOperation, result lineResult) lineResult {
	if len(operations) == 0 {
		return result
	}
	return r.withAppliedRedirects(operations, func(Runtime) lineResult { return result })
}

func (r Runtime) assignmentStatus(assignments []assignment, mark int) int {
	if status := r.assignVars(assignments); status != 0 {
		return status
	}
	if status, performed := r.expansion.substitutionStatusSince(mark); performed {
		return status
	}
	return 0
}

// replaceCommandTokens swaps the command and its arguments for a new word list,
// keeping the leading assignment tokens that sit before commandStart.
func replaceCommandTokens(tokens []shellToken, commandStart int, words []string) []shellToken {
	rebuilt := make([]shellToken, 0, commandStart+len(words))
	rebuilt = append(rebuilt, tokens[:commandStart]...)
	for _, word := range words {
		rebuilt = append(rebuilt, shellToken{kind: tokenWord, value: word})
	}
	return rebuilt
}

func (r Runtime) expandRedirectOperations(ctx context.Context, operations []redirectOperation, savedStatus int) ([]redirectOperation, bool) {
	for index, operation := range operations {
		if operation.duplicate {
			resolved, err := r.resolveDuplication(ctx, operation, savedStatus)
			if err != nil {
				fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
				return nil, false
			}
			operations[index] = resolved
			continue
		}
		if !operation.kind.takesPath() {
			if operation.kind == redirectHeredoc && operation.expand {
				operations[index].body = r.expandHeredocBody(ctx, operation.body, savedStatus)
			}
			if operation.kind == redirectHereString {
				// One word, unsplit: `cat <<< $x` with x set to `a b` feeds `a b`,
				// not `a`. Same exemption an assignment's value gets, for the same
				// reason -- the operand is a value here, not a list of arguments.
				values := r.expandingAssignment().expandCommandWord(ctx, operation.operand, savedStatus)
				// The trailing newline is part of the form: `read v <<< "one two"`
				// has to see a complete line or it reports end of input.
				operations[index].body = strings.Join(values, " ") + "\n"
			}
			continue
		}
		fields := r.expandCommandWord(ctx, operation.operand, savedStatus)
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
