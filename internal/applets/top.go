package applets

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
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

// topArgs reads the options.
//
// The letters are top's and htop's where they agree: -b batch, -n iterations, -d delay, -H
// threads, -t tree, -s sort, -f filter, -o columns. Nothing here invents a letter that either of
// them uses for something else -- -o is top's own, which names the sort field there and the field
// list here, the nearest thing to it this has.
func topArgs(args []string) (topOptions, error) {
	options := topOptions{iterations: 1, delay: topRefresh, sort: "cpu"}
	for index := 0; index < len(args); index++ {
		argument := args[index]
		value := func() (string, error) {
			if index+1 >= len(args) {
				return "", fmt.Errorf("%s requires a value", argument)
			}
			index++
			return args[index], nil
		}
		switch argument {
		case "-b":
			options.batch = true
		case "-H":
			options.threads = true
		case "-t":
			options.tree = true
		case "-n":
			text, err := value()
			if err != nil {
				return options, err
			}
			count, err := strconv.Atoi(text)
			if err != nil || count < 1 {
				return options, fmt.Errorf("invalid iteration count: %s", text)
			}
			options.iterations = count
		case "-d":
			text, err := value()
			if err != nil {
				return options, err
			}
			seconds, err := strconv.ParseFloat(text, 64)
			if err != nil || seconds <= 0 {
				return options, fmt.Errorf("invalid delay: %s", text)
			}
			options.delay = time.Duration(seconds * float64(time.Second))
		case "-s":
			text, err := value()
			if err != nil {
				return options, err
			}
			// `help` in place of a column name prints the glossary, which is htop's
			// `--sort-key help` and worth having for the same reason: it puts the
			// terminology where it can be read, grepped and pasted without opening the
			// drawn form. That matters more here than on Linux, because the drawn form
			// needs a real console and several terminals on this platform are not one.
			if text == "help" {
				options.glossary = true
				return options, nil
			}
			if _, ok := columnByKey(text); !ok {
				return options, fmt.Errorf("unknown sort column: %s", text)
			}
			options.sort = text
		case "-o":
			text, err := value()
			if err != nil {
				return options, err
			}
			if text == "help" {
				options.glossary = true
				return options, nil
			}
			options.columns = strings.Split(text, ",")
			if _, unknown := resolveColumns(options.columns); len(unknown) > 0 {
				return options, fmt.Errorf("unknown column: %s", strings.Join(unknown, ", "))
			}
		case "-f":
			text, err := value()
			if err != nil {
				return options, err
			}
			options.filter = text
		default:
			if strings.HasPrefix(argument, "-") {
				return options, fmt.Errorf("unsupported top option: %s", argument)
			}
			// Not an option, so not an option problem. top watches the whole machine
			// and has nothing to point at a file with, and saying that is more use
			// than calling a path an unsupported option.
			return options, fmt.Errorf("takes no operands: %s", argument)
		}
	}
	return options, nil
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
	model.Filter = options.filter
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
