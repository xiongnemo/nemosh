package runtime

// An error in a builtin that is not special ends that builtin and not the shell. busybox's
// evalcommand catches an error raised while such a builtin runs ("exception_type == EXERROR &&
// spclbltin <= 0", shell/ash.c), and the shell goes on with the status the error left, 2. bash
// goes on too. POSIX makes a shell error fatal in a special builtin only, and `command` makes a
// special builtin a plain one (2.14). So `declare r=2`, `let r=5` and `(( r=5 ))` on a readonly
// r, and `command export r=2`, `command unset r` and `command eval r=2`, report the error and
// the script goes on, where each ended it here. A special builtin's error still ends the script,
// as busybox's does -- local and times are special there too -- and so does one in a function,
// which is no builtin.

// containedError is a builtin's status once it has run: a shell error it raised is its own
// failure when it is not special. declare and typeset, bash's alone, keep the status bash gives,
// 1; the rest take busybox's 2. bash's discard is left to take the rest of the line, as it does
// after any command: `declare -a m=([-1]=x); echo no` says nothing more.
func (r Runtime) containedError(name string, status int) int {
	if errorEndsShell(name) || r.expansion.discard || !r.shellErrorRaised() {
		return status
	}
	contained := r.shellErrorResult().status
	if (name == "declare" || name == "typeset") && status != 0 {
		return status
	}
	return contained
}

// errorEndsShell is whether an error in the builtin name ends a script: busybox's special
// builtins, which are POSIX's with local and times.
func errorEndsShell(name string) bool {
	return isSpecialBuiltin(name) || name == "local" || name == "times"
}

// plainResult is a special builtin's result under `command`, which makes it a plain one: an
// error that would have ended the script ends the builtin, with its status. `command eval
// 'echo in; r=2; echo out'` on a readonly r says in, and the script goes on with 2, as in
// busybox.
func (r Runtime) plainResult(plain bool, result lineResult) lineResult {
	if plain && result.control == flowAbort {
		r.shellErrorRaised()
		result.control = flowNone
	}
	return result
}
