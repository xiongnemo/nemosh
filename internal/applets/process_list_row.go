package applets

import (
	"cmp"
	"strings"
	"time"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// psTable is what ps reads: the system table at one instant, and how to ask about a process
// beyond it -- its owner and its command line, which need a handle on it.
type psTable struct {
	taken     time.Time
	memory    uint64
	processes []proc.Process
	details   detailSource
	// self is the account this session runs as, whose processes BSD's form lists without `a`.
	self string
}

// psRow is one process as ps prints it. Its details are asked for once, and only if a column, a
// selection or a sort wants them: opening every process to print its pid would be the expensive
// part of a cheap command.
type psRow struct {
	process proc.Process
	table   *psTable
	// prefix is what --forest or -H puts before the command, by its depth in the tree.
	prefix  string
	read    bool
	details proc.Details
}

func (r *psRow) detail() proc.Details {
	if !r.read {
		r.details, r.read = r.table.details.Lookup(r.process), true
	}
	return r.details
}

// user is the account the process runs as, or `?` where it could not be read: Windows refuses a
// process another account runs to an unelevated session, and busybox-w32's `root` for those
// would name an owner nobody asked.
func (r *psRow) user() string {
	if user := r.detail().User; user != "" {
		return user
	}
	return "?"
}

// args is the command line, or, as procps prints a process whose arguments it cannot have, the
// image name in brackets.
func (r *psRow) args() string {
	if line := r.detail().CommandLine; line != "" {
		return printableCommand(line)
	}
	return "[" + printableCommand(r.process.Name) + "]"
}

func (r *psRow) comm() string { return printableCommand(r.process.Name) }

func (r *psRow) threads() int { return r.process.Threads }

func (r *psRow) startTime() string { return psStartTime(r.process.Created, r.table.taken) }

// elapsed is how long the process has run, in whole seconds as procps counts it.
func (r *psRow) elapsed() time.Duration {
	if r.process.Created.IsZero() || r.table.taken.Before(r.process.Created) {
		return 0
	}
	return r.table.taken.Sub(r.process.Created).Truncate(time.Second)
}

func (r *psRow) cpu() time.Duration { return r.process.Kernel + r.process.User }

// cpuShare is the CPU used over the time run, in tenths of a percent: procps' %cpu, which is a
// lifetime average and not top's rate, and can pass 100 for a process with more than one thread.
func (r *psRow) cpuShare() int64 {
	seconds := int64(r.elapsed() / time.Second)
	if seconds == 0 {
		return 0
	}
	return int64(r.cpu()/time.Millisecond) / seconds
}

// cpuRatio is cpuShare unrounded, for sorting by it.
func (r *psRow) cpuRatio() float64 {
	if seconds := r.elapsed().Seconds(); seconds > 0 {
		return r.cpu().Seconds() / seconds
	}
	return 0
}

// memoryShare is the resident set against physical memory, in tenths of a percent.
func (r *psRow) memoryShare() int64 {
	total := r.table.memory / 1024
	if total == 0 {
		return 0
	}
	return int64(min(r.process.WorkingSet/1024*1000/total, 999))
}

// stat is the state and procps' marks after it: `<` for a priority class above normal and `N`
// for one below, which is what those two mean, and `l` for more than one thread. The state is
// derived from the threads' (see proc.State); Windows keeps none for a process.
func (r *psRow) stat() string {
	text := r.state()
	switch {
	case r.process.Priority > 8:
		text += "<"
	case r.process.Priority > 0 && r.process.Priority < 8:
		text += "N"
	}
	if r.process.Threads > 1 {
		text += "l"
	}
	return text
}

// state is the process state alone, `?` where no thread's could be read.
func (r *psRow) state() string {
	if r.process.State == 0 {
		return string(rune(proc.StateUnknown))
	}
	return string(rune(r.process.State))
}

// printableCommand is text with each control character a `?`, as procps escapes it, so that one
// process is one line.
func printableCommand(text string) string {
	if !strings.ContainsFunc(text, isControl) {
		return text
	}
	return strings.Map(func(r rune) rune {
		if isControl(r) {
			return '?'
		}
		return r
	}, text)
}

func isControl(r rune) bool { return r < 0x20 || r == 0x7f }

// compareText and compareNumber are the two ways a sort key compares.
func compareText(get func(*psRow) string) func(a, b *psRow) int {
	return func(a, b *psRow) int { return strings.Compare(get(a), get(b)) }
}

func compareNumber[N cmp.Ordered](get func(*psRow) N) func(a, b *psRow) int {
	return func(a, b *psRow) int { return cmp.Compare(get(a), get(b)) }
}
