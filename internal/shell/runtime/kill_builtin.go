package runtime

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// kill is a builtin, and busybox's is too, for exactly one reason: `%N` names a
// job and only the shell has the job table.
//
// busybox's killcmd (shell/ash.c:4787) does nothing except translate `%N` into
// that job's pids and hand the result to the ordinary kill. Here `%N` goes to the
// job's record, which knows how to reach it: a job that is a process through its
// control pipe and its Job Object (job_process_signal.go), and one that is a
// goroutine -- NEMOSH_JOBS=goroutine -- through its inbox and its own context. An
// external command in a goroutine job is launched with exec.CommandContext under
// that context, so cancelling it terminates the real process.
//
// A pid operand is killed for real, through internal/proc -- the same code the
// pkill applet uses, so the two cannot disagree about what killing means.
//
// Its failures have busybox's statuses, in this shell's words: no operand is 1, where bash
// prints its usage with 2; and the status is how many operands failed, as busybox counts
// them -- 255 at most here, where 256 would have read as success.
func (r Runtime) killBuiltin(args []string) int {
	signal, operands, err := parseKillSignal(args)
	if err != nil {
		// 1, as both references answer a signal they will not send.
		fmt.Fprintf(r.streams.Stderr, "kill: %v\n", err)
		return 1
	}
	if signal == listSignals {
		return r.listKillSignals(operands)
	}
	if len(operands) == 0 {
		fmt.Fprintln(r.streams.Stderr, "kill: expected a job or a process id")
		return 1
	}
	// Every job is found before anything is sent, as busybox's killcmd turns them all into pids
	// first: a spec that names none is its getjob error, 2, and nothing is signalled.
	for _, operand := range operands {
		if !strings.HasPrefix(operand, "%") {
			continue
		}
		if _, ok := r.jobScope.lookup(r.jobScope.resolveJobSpec(operand)); !ok {
			fmt.Fprintf(r.streams.Stderr, "kill: %s\n", noSuchJob(operand))
			return 2
		}
	}
	failures := 0
	for _, operand := range operands {
		if err := r.killOne(operand, signal); err != nil {
			fmt.Fprintf(r.streams.Stderr, "kill: %v\n", err)
			failures++
		}
	}
	return min(failures, 255)
}

func (r Runtime) killOne(operand string, signal int) error {
	if strings.HasPrefix(operand, "%") {
		return r.killJob(operand, signal)
	}
	pid, err := strconv.Atoi(operand)
	if err != nil {
		// The operand, quoted, as busybox names it.
		return fmt.Errorf("invalid pid '%s'", operand)
	}
	// A job that is a process goes through its record, so its status is 128+n and
	// `jobs` names the signal, as for `kill %N`. One that has ended is a pid with no
	// process, said of the pid that was written.
	if id, found := r.jobScope.lookupPID(pid); found {
		err := r.killJob("%"+strconv.FormatUint(uint64(id), 10), signal)
		if errors.Is(err, errJobEnded) {
			return fmt.Errorf("cannot signal pid %d: %w", pid, proc.ErrNoSuchProcess)
		}
		return err
	}
	if pid == os.Getpid() && signal != 0 && r.killSelf(signal) {
		return nil
	}
	return proc.Terminate(pid, signal)
}

// killSelf takes `kill -TERM $$`, the shell's signal to itself, as bash takes one: its trap
// runs once the kill has finished, and one nothing catches ends the script with 128+n, the
// EXIT trap still running. It went to proc.Terminate, which ended the process on the spot
// with every trap unrun and an exit code a parent read as 0. busybox-w32 ignores it. A
// prompt ignores TERM, INT and QUIT it has no trap for, as bash's does, so `kill $$` typed
// there does not close the terminal. It reports whether it took the signal, which a
// subshell, having no inbox, cannot.
func (r Runtime) killSelf(signal int) bool {
	if r.interactive.session && r.subshellDepth == 0 {
		if r.signals.offer(signal) {
			return true
		}
		switch signal {
		case 2, 3, 15:
			return true
		}
	}
	return r.signals.raise(signal)
}

// killJob sends one background job a signal.
//
// A job that has set a trap for the signal, or an empty one to ignore it, is handed it
// and carries on, as in bash (signal_inbox.go). Otherwise the job stops, and it reports
// which signal stopped it -- 143 for TERM, `Terminated` in `jobs` -- as busybox's jobs
// do. busybox never runs the trap: it cannot deliver a signal to one, so every kill
// there ends the job.
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
	record, ok := r.jobScope.lookup(r.jobScope.resolveJobSpec(spec))
	if !ok {
		return errors.New(noSuchJob(spec))
	}
	select {
	case <-record.done:
		return fmt.Errorf("%s: %w", spec, errJobEnded)
	default:
	}
	if signal == 0 {
		return nil
	}
	// Offered first, since the job may have a trap for it: then the trap runs and the job
	// carries on. KILL is never offered, because nothing catches it.
	if signal != 9 && record.deliver != nil && record.deliver(signal) {
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
		return fmt.Errorf("%s: %w", spec, errJobEnded)
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
	// `--` ends the options, so `kill -- -123` names a group, in both references; it was a
	// signal named "-".
	if args[0] == "--" {
		return signal, args[1:], nil
	}
	spec := args[0][1:]
	// -L is bash's spelling of -l.
	if spec == "l" || spec == "L" {
		return listSignals, args[1:], nil
	}
	// `-s NAME`, the POSIX spelling, which busybox has, and bash's `-n NUMBER`; each was an
	// unknown signal named s or n.
	if spec == "n" && len(args) == 1 {
		return 0, nil, fmt.Errorf("-n: option requires an argument")
	}
	if (spec == "s" || spec == "n") && len(args) > 1 {
		number, err := killSignalNumber(args[1])
		if err != nil {
			return 0, nil, err
		}
		return number, args[2:], nil
	}
	number, err := killSignalNumber(spec)
	if err != nil {
		return 0, nil, err
	}
	return number, args[1:], nil
}

// killSignalNumber is proc.ParseSignal, with the name it does not know quoted as busybox
// quotes it.
func killSignalNumber(spec string) (int, error) {
	number, err := proc.ParseSignal(spec)
	if errors.Is(err, proc.ErrUnknownSignal) {
		return 0, fmt.Errorf("invalid signal '%s'", spec)
	}
	return number, err
}

// errJobEnded is a job that has finished, which no signal reaches.
var errJobEnded = errors.New("the job has already ended")

// defaultKillSignal is TERM, as everywhere.
const defaultKillSignal = 15
