package runtime

import "context"

// runSimpleWords expands a simple command's words and makes its redirections in POSIX 2.9.1's
// order, as busybox's evalcommand has them: the command name and its arguments, then the
// redirections, their targets expanded and the files opened, and then the assignments in front
// of the command, inside the redirections. mark is where the command's substitutions begin.
//
// The targets were expanded before anything and the redirections made after everything, so
// `echo $((i+=1)) > f$i` wrote f0, where both references write f1, and `x=1 >$(echo f; false)`
// was 0, where both take the status of the substitution in the target, 1. An assignment's
// substitution wrote around the redirections: `x=$(cmd) 2>/dev/null` let cmd's errors through,
// where busybox sends them to /dev/null with the rest. bash expands the assignments before it
// makes the redirections, and lets them through.
func (r Runtime) runSimpleWords(ctx context.Context, command []word, operations []redirectOperation, mark, savedStatus int) lineResult {
	prefix := assignmentPrefix(command)
	if prefix == 1 && len(command) == 1 && len(operations) == 0 {
		if result, done := r.runLoneAssignment(ctx, command[0], mark, savedStatus); done {
			return result
		}
	}
	arguments := r.expandCommandArguments(ctx, command[prefix:], savedStatus)
	if r.shellErrorRaised() {
		return r.shellErrorResult()
	}
	operations, ok := r.expandRedirectOperations(ctx, operations, savedStatus)
	if r.shellErrorRaised() {
		return r.shellErrorResult()
	}
	if !ok {
		return lineResult{status: 1}
	}
	run := func(runner Runtime, operations []redirectOperation) lineResult {
		leading := runner.expandLeadingAssignments(ctx, command[:prefix], savedStatus)
		expanded := arguments
		if len(leading) > 0 {
			expanded = append(leading, arguments...)
		}
		return runner.runExpandedWords(ctx, r, command, expanded, len(leading), operations, mark, savedStatus)
	}
	// exec's redirections are its own to make: without a command they outlive it.
	if prefix == 0 || len(operations) == 0 || isExecCommand(arguments) {
		return run(r, operations)
	}
	// Assignments with no command, in bash mode, are made whether their redirections can be or
	// not, as bash makes them: `x=1 >/no/such/dir/f` sets x, status 1.
	if len(arguments) == 0 && r.BashMode() {
		result := run(r, nil)
		if result.control != flowNone {
			return result
		}
		return r.withAppliedRedirectsFor(false, operations, func(Runtime) lineResult { return result })
	}
	special := len(arguments) > 0 && isSpecialBuiltin(arguments[0].value)
	return r.withAppliedRedirectsFor(special, operations, func(redirected Runtime) lineResult {
		return run(redirected, nil)
	})
}

// isExecCommand is whether the command is exec, run through command or not.
func isExecCommand(arguments []shellToken) bool {
	if len(arguments) == 0 {
		return false
	}
	return throughCommandPrefix(tokenValues(arguments))[0] == "exec"
}

// runLoneAssignment runs a command that is one assignment and nothing else, as most of a loop's
// lines are: its word expanded once and assigned, with nothing bound for later words to see and
// then undone, and no list of tokens made of it. It says it ran nothing when the word comes to
// anything but one assignment, which the whole path then takes.
func (r Runtime) runLoneAssignment(ctx context.Context, item word, mark, savedStatus int) (lineResult, bool) {
	values := r.expandAssignmentWord(ctx, item, false, savedStatus)
	if r.shellErrorRaised() {
		return r.shellErrorResult(), true
	}
	if len(values) != 1 || !isAssignment(keepEmptySubscript(values[0])) {
		return lineResult{}, false
	}
	r.traceCommand(ctx, values, savedStatus)
	target, value, _ := cutAssignment(keepEmptySubscript(values[0]))
	name, appended := splitAssignmentTarget(target)
	status := r.assignmentStatus([]assignment{{name: name, value: value, appended: appended}}, mark)
	r.vars["_"] = ""
	return r.abortOnShellError(lineResult{status: status}), true
}
