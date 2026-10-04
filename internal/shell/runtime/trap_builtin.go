package runtime

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/xiongnemo/nemosh/internal/proc"
	"github.com/xiongnemo/nemosh/internal/shellquote"
)

// trap implements the POSIX `trap` builtin over the conditions this shell
// promises: EXIT and INT (docs/design/v0-readiness.md, P0.4); the rest of busybox-w32's
// signals, HUP, QUIT, ILL, FPE, SEGV, PIPE, TERM and ABRT, which `kill` can send a
// background job or the shell (signal_inbox.go), PIPE also raised by a write into a pipe
// no one reads (pipe_trap.go); and ERR, which is
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
		return r.listKillSignals(nil)
	case len(args) > 0 && args[0] == "-p":
		return r.printTraps(args[1:])
	case len(args) > 0 && args[0] == "-P":
		return r.printTrapActions(args[1:])
	case len(args) > 0 && len(args[0]) > 1 && args[0][0] == '-':
		// Any other option is refused, as in both references, and ends the script as a
		// special builtin's usage error does in busybox. `trap -1 EXIT` armed EXIT with a
		// command named -1, which ran as the shell exited.
		fmt.Fprintf(r.streams.Stderr, "%strap: illegal option %s\n", r.diagnosticPrefix(), args[0])
		r.raiseShellError()
		return 2
	}
	if len(args) == 0 {
		return r.listTraps()
	}
	action, conditions := "-", args
	// An unsigned number first is a condition, not an action, and every operand is then one
	// to reset, as POSIX has it and both references read it: `trap 0 2` armed INT with a
	// command named 0.
	if len(args) > 1 && !isDigits(args[0]) {
		action, conditions = args[0], args[1:]
	}
	status := 0
	for _, condition := range conditions {
		name, ok := trapConditionName(condition)
		if !ok {
			// bash's wording, which busybox copies deliberately.
			fmt.Fprintf(r.streams.Stderr, "%strap: %s: invalid signal specification\n", r.diagnosticPrefix(), condition)
			status = 1
			continue
		}
		if name == "" {
			// A real signal this shell cannot deliver. Saying it is invalid
			// would send the reader hunting for a typo that is not there.
			fmt.Fprintf(r.streams.Stderr, "%strap: %s: not supported by this shell\n", r.diagnosticPrefix(), condition)
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
				fmt.Fprintf(r.streams.Stdout, "trap -- %s %s\n", shellquote.Ash(action), name)
			}
		}
	}
	return 0
}

// listTraps lists the armed handlers as both references order them: EXIT, the signals by
// number, then bash's DEBUG, ERR and RETURN, of which busybox has ERR, last. They were in
// the order of their names, ERR before EXIT.
func (r Runtime) listTraps() int {
	names := slices.SortedFunc(maps.Keys(r.traps), func(a, b trapName) int { return trapRank(a) - trapRank(b) })
	for _, name := range names {
		fmt.Fprintf(r.streams.Stdout, "trap -- %s %s\n", shellquote.Ash(r.traps[name]), name)
	}
	return 0
}

// trapRank is where a trap comes in the listing.
func trapRank(name trapName) int {
	switch name {
	case trapDEBUG:
		return 1000
	case trapERR:
		return 1001
	case trapRETURN:
		return 1002
	}
	return signalNumber(name)
}

// trapConditionName maps an operand to the condition it names. The second
// result distinguishes an operand that is not a signal at all from one that is
// a real signal this shell does not implement; the first is empty for the
// latter. POSIX allows the signal number as well as the name, and 0 for EXIT.
//
// A signal's name is read in any case, with SIG in front or not, as both references read it:
// `trap - int` left INT's trap armed. ERR and RETURN, which are not signals, only as written,
// as busybox has ERR; DEBUG, which busybox has not got, in any case but without SIG, as bash.
func trapConditionName(operand string) (trapName, bool) {
	switch operand {
	case "ERR":
		return trapERR, true
	case "RETURN":
		return trapRETURN, true
	}
	if strings.EqualFold(operand, "DEBUG") {
		return trapDEBUG, true
	}
	name := strings.TrimPrefix(strings.ToUpper(operand), "SIG")
	if name == "EXIT" || name == "0" {
		return trapExit, true
	}
	if number, err := proc.ParseSignal(name); err == nil {
		if trap, ok := signalTraps[number]; ok {
			return trap, true
		}
	}
	if _, err := strconv.Atoi(operand); err == nil || slices.Contains(portableSignalNames, name) {
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
