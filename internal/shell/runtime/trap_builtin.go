package runtime

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
)

// trap implements the POSIX `trap` builtin over the two conditions this shell
// promises, EXIT and INT (docs/design/v0-readiness.md, P0.4).
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
	if len(args) == 0 {
		return r.listTraps()
	}
	action, conditions := "-", args
	if len(args) > 1 {
		action, conditions = args[0], args[1:]
	}
	status := 0
	for _, condition := range conditions {
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
		if action == "-" {
			delete(r.traps, name)
			continue
		}
		r.traps[name] = action
	}
	return status
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
// so a script trapping TERM is told the truth -- that this shell does not
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
