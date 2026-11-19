package applets

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// `top` -- a process monitor for a session with no privileges.
//
// It exists because Windows has no usable one. `ntop` shows a list and little else; `btop++`
// asks to be elevated and refuses to run unelevated; Task Manager is not a terminal program. The
// gap is specifically *rich and unprivileged*, and internal/proc showed it is a gap in the tools
// rather than in the platform: the system table answers with CPU, memory, threads, handles,
// parentage and IO for every process on the machine without opening one of them.
//
// Two shapes, and the difference is the destination rather than a preference. With a terminal it
// draws; without one -- into a pipe, a file, or a test -- it prints one plain sample and exits,
// which is `top -b` on every other system and is the only form anything automated can consume.
// A monitor that insisted on a terminal would be unusable from a script, and a monitor that
// never drew would not be the thing that is missing.

// topRefresh is the default gap between samples.
//
// One second, as top and htop use. Faster looks livelier and reports mostly noise: the kernel's
// own accounting moves on a 15.6 ms tick, so a 200 ms sample divides two small numbers.
const topRefresh = time.Second

// topOptions is what the command line asked for.
type topOptions struct {
	batch      bool
	iterations int
	delay      time.Duration
	// sort is a column key, so `top -s mem` starts sorted by memory.
	sort string
	// filter narrows the list before anything is drawn, which is how a script asks about one
	// program.
	filter  string
	threads bool
	tree    bool
	// columns overrides the layout, which is how a script asks for exactly the figures it
	// wants rather than whatever fits the window it was given.
	columns []string
	// glossary asks for the columns and what they mean, on standard output.
	glossary bool
	// pids and users are procps' -p and -u, which narrow the list as the filter does; direction is
	// -o FIELD's +, high to low, or -, low to high, and 0 for the column's own.
	pids      []int
	users     []string
	direction int
}

func newTopApplet() Applet {
	return simpleApplet{name: "top", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		options, err := topArgs(args)
		if err != nil {
			return err
		}
		// Before anything is sampled: the glossary answers a question about the table
		// rather than about the machine.
		if options.glossary {
			return writeTopGlossary(stdout)
		}
		// The destination decides the form, exactly as it does for `ls`: a terminal gets
		// the interactive table, anything else gets plain text. -b forces the plain form
		// even on a terminal, which is what a script running under a terminal needs.
		if options.batch {
			return runTopBatch(ctx, options, stdout)
		}
		if !stdoutIsTerminal(stdout) {
			// Said out loud, because the silence was the real defect: someone who
			// typed `top` expecting a drawn table and got four lines of text has no
			// way to tell a deliberate choice from a broken one. This is the one
			// sentence that distinguishes them.
			fmt.Fprintln(stderr, "top: output is not a terminal, printing one sample; -b to silence this")
			return runTopBatch(ctx, options, stdout)
		}
		return runTopInteractive(ctx, options, stdin, stdout, stderr)
	}}
}

// topSession holds what a monitor needs between samples: the sampler's buffers, the previous
// snapshot to take rates against, and the cache of what handles were willing to say.
type topSession struct {
	sampler  *proc.Sampler
	details  *proc.DetailCache
	previous proc.Snapshot
	model    topModel
	// layoutFixed says the columns were named on the command line, so widening the window must
	// not silently replace them: -o is an instruction, and the width rule is only a default.
	layoutFixed bool
}

func newTopSession(options topOptions) *topSession {
	columns, _ := resolveColumns(topDefaultColumns)
	fixed := options.columns != nil
	if fixed {
		columns, _ = resolveColumns(options.columns)
	}
	model := newTopModel(columns)
	model.setSort(options.sort)
	if options.direction != 0 {
		model.Descending = options.direction > 0
	}
	model.Filter = options.filter
	model.Users = options.users
	if len(options.pids) > 0 {
		model.Pids = map[int]bool{}
		for _, pid := range options.pids {
			model.Pids[pid] = true
		}
	}
	model.Tree = options.tree
	model.Threads = options.threads
	return &topSession{
		sampler:     proc.NewSampler(),
		details:     proc.NewDetailCache(),
		model:       model,
		layoutFixed: fixed,
	}
}

// sample takes the next reading and returns the rows to show.
//
// The first call has nothing to compare against, so every CPU figure is zero -- which is what top
// and htop both do, and is why they are usually run for more than one iteration.
func (s *topSession) sample() (proc.Snapshot, proc.Rates, []topRow, error) {
	snapshot, err := s.sampler.Sample(s.model.Threads)
	if err != nil {
		return proc.Snapshot{}, proc.Rates{}, nil, err
	}
	rates := proc.Between(s.previous, snapshot)
	s.previous = snapshot
	live := make(map[int]bool, len(snapshot.Processes))
	for _, process := range snapshot.Processes {
		live[process.PID] = true
	}
	s.details.Forget(live)
	return snapshot, rates, s.model.rows(snapshot, rates, s.details), nil
}
