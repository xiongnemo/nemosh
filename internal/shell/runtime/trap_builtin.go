package runtime

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
)

// trap implements the POSIX `trap` builtin over the conditions this shell
// promises: EXIT and INT (docs/design/v0-readiness.md, P0.4); HUP, QUIT and TERM,
// which `kill` can send a background job (signal_inbox.go); and ERR, which is
// not a signal at all and so needs nothing Windows lacks -- busybox and bash both
// have it, and they agree on every case measured (errTrapTriggers).
//
//	trap                     list the armed handlers, re-readable as input
//	trap ACTION COND...      arm ACTION on each condition
//	trap - COND...           reset each condition to its default
//	trap COND...             the same reset: with one operand there is no
//	                         action word to read
//	trap '' COND...          ignore each condition
//
// The action-versus-condition rule and the lone-dash reset are busybox ash's,
// from trapcmd: the action is only taken from the front when another operand
// follows it, and `LONE_DASH(action)` clears the entry rather than storing a
// command named `-`. Storing it was the old behaviour, so `trap - EXIT` left
// the handler armed and printed `-: not found` when the shell exited.
func (r Runtime) trap(args []string) int {
	switch {
	case len(args) > 0 && args[0] == "--":
		args = args[1:]
	case len(args) > 0 && args[0] == "-l":
		// bash's `trap -l` is `kill -l`, and so is this.
		return r.listKillSignals()
	case len(args) > 0 && args[0] == "-p":
		return r.printTraps(args[1:])
	}
	if len(args) == 0 {
		return r.listTraps()
	}
	action, conditions := "-", args
	if len(args) > 1 {
		action, conditions = args[0], args[1:]
	}
	status := 0
	for _, condition := range conditions {
		if condition == "DEBUG" {
			// Named rather than called invalid: bash has it, and what it needs is missing.
			fmt.Fprintln(r.streams.Stderr, "trap: DEBUG: not implemented: it runs before every command "+
				"with $BASH_COMMAND, the command as written, and a command here keeps no written form")
			status = 1
			continue
		}
		name, ok := trapConditionName(condition)
		if !ok {
			// bash's wording, which busybox copies deliberately.
			fmt.Fprintf(r.streams.Stderr, "trap: %s: invalid signal specification\n", condition)
			status = 1
			continue
		}
		if name == "" {
			// A real signal this shell cannot deliver. Saying it is invalid
			// would send the reader hunting for a typo that is not there.
			fmt.Fprintf(r.streams.Stderr, "trap: %s: not supported by this shell\n", condition)
			status = 1
			continue
		}
		r.setTrap(name, action)
	}
	return status
}

// printTraps is bash's `trap -p [condition...]`: the armed handlers, re-readable, all of
// them or the ones named. busybox refuses the option; here it was taken for an action, so
// `trap -p RETURN` armed RETURN with a command named -p.
func (r Runtime) printTraps(conditions []string) int {
	if len(conditions) == 0 {
		return r.listTraps()
	}
	for _, condition := range conditions {
		if name, ok := trapConditionName(condition); ok && name != "" {
			if action, set := r.traps[name]; set {
				fmt.Fprintf(r.streams.Stdout, "trap -- %s %s\n", singleQuoteForReuse(action), name)
			}
		}
	}
	return 0
}

func (r Runtime) listTraps() int {
	for _, name := range slices.Sorted(maps.Keys(r.traps)) {
		fmt.Fprintf(r.streams.Stdout, "trap -- %s %s\n", singleQuoteForReuse(r.traps[name]), name)
	}
	return 0
}

// trapConditionName maps an operand to the condition it names. The second
// result distinguishes an operand that is not a signal at all from one that is
// a real signal this shell does not implement; the first is empty for the
// latter. POSIX allows the signal number as well as the name, and 0 for EXIT.
func trapConditionName(operand string) (trapName, bool) {
	switch operand {
	case "EXIT", "SIGEXIT", "0":
		return trapExit, true
	case "INT", "SIGINT", "2":
		return trapINT, true
	case "HUP", "SIGHUP", "1":
		return trapHUP, true
	case "QUIT", "SIGQUIT", "3":
		return trapQUIT, true
	case "TERM", "SIGTERM", "15":
		return trapTERM, true
	case "ERR":
		return trapERR, true
	case "RETURN":
		return trapRETURN, true
	}
	if _, err := strconv.Atoi(operand); err == nil {
		return "", true
	}
	if slices.Contains(portableSignalNames, operand) {
		return "", true
	}
	return "", false
}

// The signal names POSIX XCU requires `kill -l` to know. Nemosh recognises them
// so a script trapping USR1 is told the truth -- that this shell does not
// deliver it -- rather than being told the name is wrong.
var portableSignalNames = []string{
	"ABRT", "ALRM", "BUS", "CHLD", "CONT", "FPE", "HUP", "ILL", "KILL", "PIPE",
	"POLL", "PROF", "QUIT", "SEGV", "STOP", "SYS", "TERM", "TRAP", "TSTP",
	"TTIN", "TTOU", "URG", "USR1", "USR2", "VTALRM", "XCPU", "XFSZ",
	"SIGABRT", "SIGALRM", "SIGBUS", "SIGCHLD", "SIGCONT", "SIGFPE", "SIGHUP",
	"SIGILL", "SIGKILL", "SIGPIPE", "SIGPOLL", "SIGPROF", "SIGQUIT", "SIGSEGV",
	"SIGSTOP", "SIGSYS", "SIGTERM", "SIGTRAP", "SIGTSTP", "SIGTTIN", "SIGTTOU",
	"SIGURG", "SIGUSR1", "SIGUSR2", "SIGVTALRM", "SIGXCPU", "SIGXFSZ",
}
