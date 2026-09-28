package runtime

import "context"

func (r Runtime) executeSimpleCommand(ctx context.Context, command simpleCommand, savedStatus int) lineResult {
	r.enterLine(command.line)
	r.enterCommand(command)
	return r.runParsedWords(ctx, command.words, cloneRedirects(command.redirects), savedStatus)
}

// enterCommand makes command the one running, $BASH_COMMAND, as bash has it -- except while a
// trap runs, when it stays the command the trap came in on: an ERR trap's is the command that
// failed and an EXIT trap's the last one run. It was never set.
func (r Runtime) enterCommand(command simpleCommand) {
	if r.expansion != nil && len(r.trapRunning) == 0 {
		r.expansion.command, r.expansion.ran = command, true
	}
}

// bashCommand is $BASH_COMMAND: the running command as written, from the parse, as bash
// prints it for `declare -f`.
func (r Runtime) bashCommand() string {
	if r.expansion == nil || !r.expansion.ran {
		return ""
	}
	var printer scriptPrinter
	return printer.command(r.expansion.command)
}

func cloneRedirects(source []redirectOperation) []redirectOperation {
	return append([]redirectOperation(nil), source...)
}
