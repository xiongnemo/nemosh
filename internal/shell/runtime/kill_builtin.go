package runtime

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/xiongnemo/nemosh/internal/proc"
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
// A pid operand is killed for real, through internal/proc -- the same code the
// pkill applet uses, so the two cannot disagree about what killing means.
func (r Runtime) killBuiltin(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(r.streams.Stderr, "kill: expected a job or a process id")
		return 2
	}
	signal, operands, err := parseKillSignal(args)
	if err != nil {
		// 1, as both references answer a signal they will not send.
		fmt.Fprintf(r.streams.Stderr, "kill: %v\n", err)
		return 1
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
		return r.killJob(operand, signal)
	}
	pid, err := strconv.Atoi(operand)
	if err != nil {
		// busybox's wording, which names the operand rather than the option.
		return fmt.Errorf("illegal pid: %s", operand)
	}
	// A job that is a process goes through its record, so its status is 128+n and
	// `jobs` names the signal, as for `kill %N`.
	if id, found := r.jobScope.lookupPID(pid); found {
		return r.killJob("%"+strconv.FormatUint(uint64(id), 10), signal)
	}
	return proc.Terminate(pid, signal)
}

// killJob stops one background job.
//
// Every signal cancels, and saying so is better than pretending to tell TERM
// from KILL: a goroutine has no handler to run, so the distinction would be a
// promise this cannot keep. What it can promise is that the job stops, and that
// it reports which signal stopped it -- 143 for TERM, `Terminated` in `jobs` --
// as busybox's jobs do, whose targets have no handler run either.
//
// **Except zero, which only asks.** `kill -0 $pid` is how a script tests whether
// its job is still alive -- `while kill -0 $pid; do sleep 1; done` -- and it used
// to cancel the job it was asking about, then keep answering yes for as long as
// the record lasted, so that loop never ended. It answers now, and changes
// nothing.
//
// **A job that has ended is refused, whatever the signal.** Both references
// accept `kill %1` for a job that has finished but not yet been reported; they
// refuse `kill $pid` once the process is gone. Here `$!` *is* `%1`, so the
// second is the one a script is actually writing, and it is the same answer
// `kill PID` gives for a process that has exited.
func (r Runtime) killJob(spec string, signal int) error {
	value, err := strconv.ParseUint(strings.TrimPrefix(spec, "%"), 10, 64)
	if err != nil || value == 0 {
		return fmt.Errorf("invalid job: %s", spec)
	}
	record, ok := r.jobScope.lookup(jobID(value))
	if !ok {
		return fmt.Errorf("%s: no such job", spec)
	}
	select {
	case <-record.done:
		return fmt.Errorf("%s: the job has already ended", spec)
	default:
	}
	if signal == 0 {
		return nil
	}
	if record.cancel == nil {
		// A job registered without a cancel is one the shell cannot reach, which
		// would be a defect here rather than a user error -- so it says so
		// instead of reporting success.
		return fmt.Errorf("%s: cannot be signalled", spec)
	}
	// Noted before the cancel, so the status the job ends with is the signal's.
	if !r.jobScope.markSignalled(record, signal) {
		return fmt.Errorf("%s: the job has already ended", spec)
	}
	record.cancel()
	return nil
}

// listSignals is the sentinel for `kill -l`, which takes no operand.
const listSignals = -1

// parseKillSignal reads the leading signal option, if there is one.
//
// Both spellings busybox accepts, `-9` and `-TERM`, read by proc.ParseSignal --
// the table pkill and killall read too, so the three cannot disagree about which
// signals exist.
func parseKillSignal(args []string) (int, []string, error) {
	signal := defaultKillSignal
	if len(args) == 0 || !strings.HasPrefix(args[0], "-") || args[0] == "-" {
		return signal, args, nil
	}
	spec := args[0][1:]
	if spec == "l" {
		return listSignals, args[1:], nil
	}
	number, err := proc.ParseSignal(spec)
	if err != nil {
		return 0, nil, err
	}
	return number, args[1:], nil
}

// listKillSignals lists what the shell can act on, not the whole POSIX set: a
// name it would accept and then ignore would be worse than one it refuses.
func (r Runtime) listKillSignals() int {
	for _, signal := range proc.Signals() {
		fmt.Fprintf(r.streams.Stdout, "%2d) SIG%s\n", signal.Number, signal.Name)
	}
	return 0
}

// defaultKillSignal is TERM, as everywhere.
const defaultKillSignal = 15
