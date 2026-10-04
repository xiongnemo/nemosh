package proc

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// The signals this build can name, for `kill`, `pkill` and `killall` alike.
//
// **One table.** There were two -- one in the kill builtin, one beside pkill -- and they agreed
// on the one thing both had wrong: each listed STOP and CONT, and each delivered them the only
// way anything is delivered here, by terminating the target. So `kill -STOP` ended the process
// it was meant to pause. busybox-w32's table has neither name (`kill -STOP` there is "bad
// signal name"), and that is the line this draws too: see ErrCannotSuspend.
//
// Short, deliberately. Windows has no signals, so each of these is a behaviour Terminate
// reproduces rather than a number a kernel understands, and a name listed here is a promise.
// It is busybox-w32's table, which has ILL, FPE, SEGV, PIPE and ABRT besides: they were an
// invalid signal to `kill` and one `trap` did not support. ABRT is AbortSignal, 22 on Windows.
var signalNumbers = map[string]int{
	"HUP": 1, "INT": 2, "QUIT": 3, "ILL": 4, "FPE": 8, "KILL": 9, "SEGV": 11, "PIPE": 13, "TERM": 15,
	"ABRT": AbortSignal,
}

// signalWords are what a job ended by each signal is reported as -- `Terminated`, not
// `Done(143)` -- in the words bash and busybox on Linux both use, libc's. busybox-w32 has its
// own table, which names all but KILL and TERM bare, `SEGV`.
var signalWords = map[int]string{
	1: "Hangup", 2: "Interrupt", 3: "Quit", 4: "Illegal instruction", 8: "Floating point exception",
	9: "Killed", 11: "Segmentation fault", 13: "Broken pipe", 15: "Terminated", AbortSignal: "Aborted",
}

// suspendSignals are the stop-and-continue family, by name and by the Linux number a script
// written there would use.
var suspendSignals = map[string]int{"CONT": 18, "STOP": 19, "TSTP": 20, "TTIN": 21, "TTOU": 22}

// ErrCannotSuspend refuses the stop-and-continue signals.
//
// Nothing beneath this shell can pause a process and resume it later: Windows has no SIGSTOP,
// NtSuspendProcess is undocumented, and SuspendThread deadlocks a target caught holding its
// heap lock (support-matrix.md, "Process control"). A STOP delivered the way every other signal
// is would *end* the target, and a CONT has nothing it could ever resume. Refusing both by name
// is the honest answer; the error says why, so it is not mistaken for a typo.
var ErrCannotSuspend = errors.New("nothing here can suspend a process, so there is no STOP and nothing for CONT to resume")

// ErrUnknownSignal is a spec that names no signal at all.
var ErrUnknownSignal = errors.New("invalid signal")

// ParseSignal reads a signal written as a number or a name, with or without `SIG`: `9`,
// `KILL` and `SIGKILL` are the same. A script writes the number and a person writes the name.
//
// Zero is accepted. It is no signal at all: it asks whether the target exists, which is what
// `kill -0 $pid` is written for -- see Terminate.
func ParseSignal(spec string) (int, error) {
	if number, err := strconv.Atoi(spec); err == nil {
		if number < 0 {
			return 0, fmt.Errorf("%w: %s", ErrUnknownSignal, spec)
		}
		// The table first, since 22 is ABRT on Windows and TTOU only where ABRT is 6.
		if number == 0 || isSignalNumber(number) {
			return number, nil
		}
		for name, suspend := range suspendSignals {
			if number == suspend {
				return 0, fmt.Errorf("%d is SIG%s: %w", number, name, ErrCannotSuspend)
			}
		}
		// A number is a signal only if the table has it, as busybox-w32 has it: `kill -9999`
		// was accepted, and ended its target as though by a signal of that number, status
		// 10127.
		return 0, fmt.Errorf("%w: %s", ErrUnknownSignal, spec)
	}
	name := strings.TrimPrefix(strings.ToUpper(spec), "SIG")
	if number, ok := signalNumbers[name]; ok {
		return number, nil
	}
	if _, ok := suspendSignals[name]; ok {
		return 0, fmt.Errorf("%s: %w", name, ErrCannotSuspend)
	}
	return 0, fmt.Errorf("%w: %s", ErrUnknownSignal, spec)
}

// isSignalNumber reports a number the table has.
func isSignalNumber(number int) bool {
	for _, known := range signalNumbers {
		if known == number {
			return true
		}
	}
	return false
}

// Signal is one entry of the table, for listing.
type Signal struct {
	Name   string
	Number int
}

// Signals lists the table in number order, which is how `kill -l` is read.
func Signals() []Signal {
	signals := make([]Signal, 0, len(signalNumbers))
	for name, number := range signalNumbers {
		signals = append(signals, Signal{Name: name, Number: number})
	}
	sort.Slice(signals, func(i, j int) bool { return signals[i].Number < signals[j].Number })
	return signals
}

// SignalWord is how something ended by signal is reported: `Killed`, `Terminated`.
func SignalWord(signal int) string {
	if word, ok := signalWords[signal]; ok {
		return word
	}
	return fmt.Sprintf("Signal %d", signal)
}
