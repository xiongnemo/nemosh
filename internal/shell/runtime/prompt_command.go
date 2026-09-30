package runtime

import "context"

// RunPromptCommand is bash's PROMPT_COMMAND, run before a session draws its primary prompt:
// the value, or each element in turn when it is an array, as bash 5.1 has it, run with $? the
// last command's. $? and $_ are what they were after it, so the prompt and the command after it
// see what they would have without it, as bash puts them back. An error in it is reported and
// ends only it; an `exit` in it ends the session. busybox has none.
func (r *Runtime) RunPromptCommand(ctx context.Context) InteractiveResult {
	commands := r.promptCommands()
	if len(commands) == 0 || r.initErr != nil {
		return InteractiveResult{Status: r.interactive.status}
	}
	lastArgument, hadLastArgument := r.vars["_"]
	for _, command := range commands {
		status, control := r.runScriptResult(ctx, command, r.currentLine(), false, r.interactive.status)
		if control == flowExit || control == flowExec {
			return InteractiveResult{Status: r.endSession(ctx, control, status), Exited: true}
		}
	}
	if hadLastArgument {
		r.vars["_"] = lastArgument
	} else {
		delete(r.vars, "_")
	}
	return InteractiveResult{Status: r.interactive.status}
}

// CommandPrompt is bash's PS0, which a session prints to stderr once it has read a command and
// before it runs it (eval.c): decoded and then expanded, as ${PS0@P} would be and as bash does
// every prompt, with \# the command about to run, \! its history number and $? the last
// command's status. Unset or empty, or empty once expanded, it prints nothing. busybox has none.
func (r Runtime) CommandPrompt(ctx context.Context, lastStatus int) string {
	value := r.vars["PS0"]
	if value == "" || r.initErr != nil {
		return ""
	}
	return r.promptTransform(ctx, value, lastStatus)
}

func (r Runtime) promptCommands() []string {
	if !r.arrays.has("PROMPT_COMMAND") {
		if value := r.vars["PROMPT_COMMAND"]; value != "" {
			return []string{value}
		}
		return nil
	}
	var commands []string
	for _, index := range r.arrays.liveIndices("PROMPT_COMMAND") {
		if value, _ := r.arrays.valueAt("PROMPT_COMMAND", index); value != "" {
			commands = append(commands, value)
		}
	}
	return commands
}

// endSession closes a session that `exit` or `exec` has ended, with the EXIT trap for exit,
// and answers the status it ends with.
func (r *Runtime) endSession(ctx context.Context, control flowControl, status int) int {
	switch control {
	case flowExit:
		r.interactive.closed = true
		status = r.runExitTrap(context.WithoutCancel(ctx), status)
		r.jobScope.seal()
	case flowExec:
		r.interactive.closed = true
		r.lifecycle.exitSuppressed = true
		r.jobScope.seal()
	}
	return status
}

// MarkSession makes this runtime a session's before it runs a command string, as `nemosh -i -c`
// runs one, bash's: an error that ends a script ends only the line it is on, as at a prompt.
func (r *Runtime) MarkSession() { r.interactive.session = true }
