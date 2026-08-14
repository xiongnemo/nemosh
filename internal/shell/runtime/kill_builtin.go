package runtime

import (
	"fmt"
	"strconv"
	"strings"
)

// kill is a builtin, and busybox's is too, for exactly one reason: `%N` names a
// job and only the shell has the job table.
//
// busybox's killcmd (shell/ash.c:4787) does nothing except translate `%N` into
// that job's pids and hand the result to the ordinary kill. Here there is nothing
// to translate into -- a background job is a goroutine, not a process, so it has
// no pid -- and what stands in for the signal is cancelling the job's own
// context. That is not a weaker substitute where it matters most: an external
// command in a background job is launched with exec.CommandContext under that
// context, so cancelling it terminates the real process.
//
// A pid operand is killed for real, through the platform: TerminateProcess on
// Windows, as busybox does (win32/process.c:909), and a signal elsewhere.
func (r Runtime) killBuiltin(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(r.streams.Stderr, "kill: expected a job or a process id")
		return 2
	}
	signal, operands, err := parseKillSignal(args)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "kill: %v\n", err)
		return 2
	}
	if signal == listSignals {
		return r.listKillSignals()
	}
	if len(operands) == 0 {
		fmt.Fprintln(r.streams.Stderr, "kill: expected a job or a process id")
		return 2
	}
	status := 0
	for _, operand := range operands {
		if err := r.killOne(operand, signal); err != nil {
			fmt.Fprintf(r.streams.Stderr, "kill: %v\n", err)
			status = 1
		}
	}
	return status
}

func (r Runtime) killOne(operand string, signal int) error {
	if strings.HasPrefix(operand, "%") {
		return r.killJob(operand)
	}
	pid, err := strconv.Atoi(operand)
	if err != nil {
		// busybox's wording, which names the operand rather than the option.
		return fmt.Errorf("illegal pid: %s", operand)
	}
	return terminateProcess(pid, signal)
}

// killJob stops one background job.
//
// Every signal cancels, and saying so is better than pretending to tell TERM
// from KILL: a goroutine has no handler to run, so the distinction would be a
// promise this cannot keep. What it can promise is that the job stops.
func (r Runtime) killJob(spec string) error {
	value, err := strconv.ParseUint(strings.TrimPrefix(spec, "%"), 10, 64)
	if err != nil || value == 0 {
		return fmt.Errorf("invalid job: %s", spec)
	}
	record, ok := r.jobScope.lookup(jobID(value))
	if !ok {
		return fmt.Errorf("%s: no such job", spec)
	}
	if record.cancel == nil {
		// A job registered without a cancel is one the shell cannot reach, which
		// would be a defect here rather than a user error -- so it says so
		// instead of reporting success.
		return fmt.Errorf("%s: cannot be signalled", spec)
	}
	record.cancel()
	return nil
}

// listSignals is the sentinel for `kill -l`, which takes no operand.
const listSignals = -1

// parseKillSignal reads the leading signal option, if there is one.
//
// Both spellings busybox accepts: `-9` and `-TERM`, with or without the `SIG`
// prefix. The number is what a script writes and the name is what a person
// writes, so refusing either would be refusing half the users.
func parseKillSignal(args []string) (int, []string, error) {
	signal := defaultKillSignal
	if len(args) == 0 || !strings.HasPrefix(args[0], "-") || args[0] == "-" {
		return signal, args, nil
	}
	spec := args[0][1:]
	if spec == "l" {
		return listSignals, args[1:], nil
	}
	if number, err := strconv.Atoi(spec); err == nil {
		if number < 0 {
			return 0, nil, fmt.Errorf("invalid signal: %s", args[0])
		}
		return number, args[1:], nil
	}
	number, ok := killSignalNumbers[strings.TrimPrefix(strings.ToUpper(spec), "SIG")]
	if !ok {
		return 0, nil, fmt.Errorf("invalid signal: %s", args[0])
	}
	return number, args[1:], nil
}

func (r Runtime) listKillSignals() int {
	for _, name := range killSignalNames {
		fmt.Fprintf(r.streams.Stdout, "%2d) SIG%s\n", killSignalNumbers[name], name)
	}
	return 0
}

// The signals worth naming. Deliberately not the whole POSIX set: this lists
// what the shell can actually act on, and a name it would accept and then ignore
// would be worse than one it refuses.
var killSignalNames = []string{"HUP", "INT", "QUIT", "KILL", "TERM", "STOP", "CONT"}

var killSignalNumbers = map[string]int{
	"HUP": 1, "INT": 2, "QUIT": 3, "KILL": 9, "TERM": 15, "STOP": 19, "CONT": 18,
}

// defaultKillSignal is TERM, as everywhere.
const defaultKillSignal = 15
