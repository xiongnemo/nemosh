package runtime

import "context"

func (r Runtime) executeSimpleCommand(ctx context.Context, command simpleCommand, savedStatus int) lineResult {
	r.enterSimpleCommand(command)
	if result, ended := r.debugTrap(ctx, savedStatus); ended {
		return result
	}
	return r.runSimpleCommand(ctx, command, savedStatus)
}

// runSimpleCommand runs a command whose DEBUG trap has run: a pipeline's stage, whose trap ran
// before the pipeline started; see debugTrapStages.
func (r Runtime) runSimpleCommand(ctx context.Context, command simpleCommand, savedStatus int) lineResult {
	if command.condition != nil {
		return r.runConditionCommand(ctx, command.condition, cloneRedirects(command.redirects), savedStatus)
	}
	return r.runParsedWords(ctx, command.words, cloneRedirects(command.redirects), savedStatus)
}

func (r Runtime) enterSimpleCommand(command simpleCommand) {
	r.enterLine(command.line)
	r.enterCommand(command)
}

// enterCommand makes command the one running, $BASH_COMMAND, as bash has it -- except while a
// trap runs, when it stays the command the trap came in on: an ERR trap's is the command that
// failed and an EXIT trap's the last one run. It was never set.
func (r Runtime) enterCommand(command simpleCommand) {
	if r.expansion != nil && len(r.trapRunning) == 0 {
		r.expansion.command, r.expansion.head, r.expansion.ran = command, "", true
	}
}

// enterHead makes a compound command's head the one running, for its DEBUG trap.
func (r Runtime) enterHead(head string) {
	if r.expansion != nil && len(r.trapRunning) == 0 {
		r.expansion.head, r.expansion.ran = head, true
	}
}

// bashCommand is $BASH_COMMAND: the running command as written, from the parse, as bash
// prints it for `declare -f`.
func (r Runtime) bashCommand() string {
	if r.expansion == nil || !r.expansion.ran {
		return ""
	}
	if r.expansion.head != "" {
		return r.expansion.head
	}
	var printer scriptPrinter
	return printer.command(r.expansion.command)
}

func cloneRedirects(source []redirectOperation) []redirectOperation {
	return append([]redirectOperation(nil), source...)
}
