package runtime

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// jobLine is the one place a job's state is put into words.
//
// `jobs` and the notice before a prompt are two ways of asking the same question about the
// same record, and a notice that called a job something `jobs` did not would be two
// accounts of one thing. The bool says whether this is final -- which is also what decides
// whether reporting it consumes the job.
//
// A job `kill` ended is named for the signal -- `Terminated`, `Killed` -- as both
// references name it, rather than `Done(143)`: done is what it was not.
//
// The line is both references': `[1]+  Running`, the mark saying which job is current (see
// job_spec.go), and with -l the pid before the state. It was `[1] Running`.
func jobLine(record *jobRecord, marker byte, long bool) (string, bool) {
	state, finished := jobCondition(record)
	prefix := fmt.Sprintf("[%d]%c  ", record.id, marker)
	if long && record.pid != 0 {
		prefix += strconv.Itoa(record.pid) + " "
	}
	return padJobLine(prefix+state) + "\n", finished
}

// jobCondition is a job's state as `jobs` names it, and whether it is final.
func jobCondition(record *jobRecord) (string, bool) {
	select {
	case <-record.done:
		state := "Done"
		if record.status != 0 {
			state += "(" + strconv.Itoa(record.status) + ")"
		}
		if record.signal != 0 {
			state = proc.SignalWord(record.signal)
		}
		return state, true
	default:
		return "Running", false
	}
}

// padJobLine pads a job's line to column 33, where both references go on to write the
// command the job runs. A command here keeps no written form, so nothing follows, as in a
// busybox script.
func padJobLine(text string) string {
	return text + strings.Repeat(" ", max(33-len(text), 1))
}

// reportSignalled says, on stderr, how each of these jobs ended if kill ended it: the word
// busybox's `wait %N` prints, `Terminated` or `Killed`. Not for INT, which neither reference
// reports -- an interrupt is something the person just did, not news to them.
//
// Only for jobs that have ended, so the signal can be read without the scope's lock.
func (r Runtime) reportSignalled(records []*jobRecord) {
	for _, record := range records {
		if record.signal != 0 && record.signal != 2 {
			fmt.Fprintln(r.streams.Stderr, proc.SignalWord(record.signal))
		}
	}
}

// jobsRequest is what `jobs` was asked for: -l and -p, as in both references, and bash's
// -r and -s, the running jobs and the stopped ones -- of which there are never any here.
type jobsRequest struct {
	long, pids, running, stopped bool
	operands                     []string
}

func parseJobsArgs(args []string) (jobsRequest, error) {
	var request jobsRequest
	for len(args) > 0 && len(args[0]) > 1 && args[0][0] == '-' {
		option := args[0]
		args = args[1:]
		if option == "--" {
			break
		}
		for _, letter := range option[1:] {
			switch letter {
			case 'l':
				request.long = true
			case 'p':
				request.pids = true
			case 'r':
				request.running = true
			case 's':
				request.stopped = true
			default:
				return request, fmt.Errorf("illegal option -%c", letter)
			}
		}
	}
	request.operands = args
	return request, nil
}

// jobs lists the job table, newest first, as busybox lists it. `-l` adds each job's pid and
// `-p` prints the pids alone. A job that is a goroutine has no pid: `-p` gives the `%N` its
// `$!` holds, which `kill` and `wait` take the same way, and `-l` leaves its line as it was.
// Operands list the jobs they name, and one that names none is status 2, as in busybox.
func (r Runtime) jobs(args []string) int {
	request, err := parseJobsArgs(args)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "%sjobs: %v\n", r.diagnosticPrefix(), err)
		return 2
	}
	records, own := r.listedJobs()
	current, previous := jobID(0), jobID(0)
	if len(records) > 0 {
		current = records[0].id
	}
	if len(records) > 1 {
		previous = records[1].id
	}
	records, status := r.selectJobs(records, request)
	if request.pids {
		return max(status, r.jobPIDs(records))
	}
	// Reported is consumed: POSIX 2.9.3 removes a job from the list once the shell has
	// reported its status, and busybox agrees -- `[1]+ Done` appears once and `jobs`
	// afterwards says nothing. This used to answer `[1] Done(1)` every time it was asked.
	var reported []*jobRecord
	for _, record := range records {
		line, finished := jobLine(record, jobMarker(record.id, current, previous), request.long)
		if finished && own {
			reported = append(reported, record)
		}
		if _, err := r.streams.Stdout.Write([]byte(line)); err != nil {
			fmt.Fprintf(r.streams.Stderr, "%sjobs: %v\n", r.diagnosticPrefix(), err)
			// Forgotten anyway: the status reached the caller's stream or it did not,
			// and either way saying it again on the next ask is not the repair.
			r.jobScope.forget(reported)
			return 1
		}
	}
	r.jobScope.forget(reported)
	return status
}

// selectJobs is the listing's records narrowed to the operands and to -r and -s, with the
// status a missing operand leaves.
func (r Runtime) selectJobs(records []*jobRecord, request jobsRequest) ([]*jobRecord, int) {
	status := 0
	if len(request.operands) > 0 {
		var named []*jobRecord
		for _, operand := range request.operands {
			record, ok := r.jobScope.lookup(r.jobScope.resolveJobSpec(operand))
			if !ok {
				fmt.Fprintf(r.streams.Stderr, "%sjobs: %s\n", r.diagnosticPrefix(), noSuchJob(operand))
				status = 2
				continue
			}
			named = append(named, record)
		}
		records = named
	}
	if request.stopped && !request.running {
		return nil, status
	}
	if request.running {
		var running []*jobRecord
		for _, record := range records {
			if _, finished := jobCondition(record); !finished {
				running = append(running, record)
			}
		}
		records = running
	}
	return records, status
}

// listedJobs are the jobs `jobs` lists, and whether they are this scope's own. A pipeline
// stage and a command substitution list the shell's, as both references do -- `jobs -p |
// wc -l` counts them, and `kill $(jobs -p)` ends them -- until they start jobs of their own.
// They were an empty table, so both idioms saw no jobs at all. Listing the shell's
// consumes nothing: its Done is still the shell's to report.
func (r Runtime) listedJobs() ([]*jobRecord, bool) {
	records := r.jobScope.snapshot()
	if len(records) > 0 || r.jobScope.outer == nil {
		return records, true
	}
	for scope := r.jobScope.outer; scope != nil; scope = scope.outer {
		if records := scope.snapshot(); len(records) > 0 {
			return records, false
		}
	}
	return nil, false
}

// jobPIDs is `jobs -p`. It reports no status, so it consumes nothing.
func (r Runtime) jobPIDs(records []*jobRecord) int {
	for _, record := range records {
		if _, err := fmt.Fprintln(r.streams.Stdout, record.identifier()); err != nil {
			fmt.Fprintf(r.streams.Stderr, "%sjobs: %v\n", r.diagnosticPrefix(), err)
			return 1
		}
	}
	return 0
}

// ReportFinishedJobs names the jobs that have ended, and forgets them.
//
// Called by the interactive loops before they draw a prompt, which is when bash and
// busybox report it too. Neither needs `set -b` for this: `set -b` asks for the report
// *immediately*, in the middle of whatever is running, and that is the part this shell has
// no channel for and goes on refusing. A prompt is a moment the loop already has.
//
// On stderr, where the launch announcement goes and where busybox puts this one -- so a
// job ending while `x=$(...)` runs cannot end up inside the captured value.
//
// Nothing is written when there is nothing to say, so the common prompt costs one lock and
// no output.
func (r Runtime) ReportFinishedJobs() {
	var reported []*jobRecord
	var notice strings.Builder
	records := r.jobScope.snapshot()
	current, previous := r.jobScope.currentAndPrevious()
	for _, record := range records {
		line, finished := jobLine(record, jobMarker(record.id, current, previous), false)
		if !finished {
			continue
		}
		notice.WriteString(line)
		reported = append(reported, record)
	}
	if len(reported) == 0 {
		return
	}
	fmt.Fprint(r.streams.Stderr, notice.String())
	r.jobScope.forget(reported)
}
