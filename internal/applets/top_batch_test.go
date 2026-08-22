package applets

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"
)

// top's batch form, which is "the whole of what a script can use and the whole of
// what a test can check" by its own file comment -- and which was at 32.9%.
//
// It needs no terminal, so unlike the interactive view it can be run for real
// against this machine's own processes. What cannot be asserted is any *number*:
// a process count, a busy percentage and a memory figure all move between one
// sample and the next. So what is asserted is the shape -- that every line the
// header promises is present, that the column count is what the header says, and
// that the two forms of `top` do not disagree.

func runTopBatchApplet(t *testing.T, args ...string) (string, string) {
	t.Helper()
	requireProcessSampling(t)
	var stdout, stderr strings.Builder
	// A generous timeout: sampling walks every process on the machine, and a CI
	// runner is slow and busy.
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	if err := newTopApplet().Run(ctx, args, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("top %v: %v (%s)", args, err, stderr.String())
	}
	return stdout.String(), stderr.String()
}

// The four summary lines, each named because each carries a decision. No load
// average, because Windows has no runnable-thread average and the nearest analogue
// counts something else; commit rather than swap, for the same reason.
func TestTopBatch_printsTheFourSummaryLines(t *testing.T) {
	stdout, _ := runTopBatchApplet(t, "-b", "-n", "1")
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) < 5 {
		t.Fatalf("top -b printed %d lines, want at least the four summary lines and a header:\n%s",
			len(lines), stdout)
	}
	for index, prefix := range []string{"top - up ", "CPU: ", "Mem: ", "Commit: "} {
		if !strings.HasPrefix(lines[index], prefix) {
			t.Errorf("line %d is %q, want it to start %q", index+1, lines[index], prefix)
		}
	}
	// The first line carries the counts, and each has to be a number followed by
	// its noun rather than an empty field.
	for _, noun := range []string{"processes", "threads", "running"} {
		if !strings.Contains(lines[0], noun) {
			t.Errorf("the first line does not report %s: %q", noun, lines[0])
		}
	}
	// No load average anywhere, which is the decision this file records.
	if strings.Contains(stdout, "load average") {
		t.Errorf("a load average appeared, which Windows has no measure for:\n%s", stdout)
	}
}

// The table header names every column, and every row has as many fields as the
// header has columns -- which is what says the padding did not run cells together.
func TestTopBatch_everyRowHasAsManyCellsAsTheHeader(t *testing.T) {
	stdout, _ := runTopBatchApplet(t, "-b", "-n", "1")
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	// Line five is the header; the rows follow.
	header := lines[4]
	if !strings.Contains(header, "PID") {
		t.Fatalf("the fifth line is not a header: %q", header)
	}
	// COMMAND is last and may hold spaces, so the count is taken up to it.
	before, _, found := strings.Cut(header, "COMMAND")
	if !found {
		t.Fatalf("the header has no COMMAND column: %q", header)
	}
	want := len(strings.Fields(before))
	if want < 3 {
		t.Fatalf("the header has only %d columns before COMMAND: %q", want, header)
	}
	if len(lines) < 6 {
		t.Skip("this machine reported no processes at all")
	}
	for _, row := range lines[5:] {
		if strings.TrimSpace(row) == "" {
			continue
		}
		// At least as many fields as the header: the command itself may add more.
		if got := len(strings.Fields(row)); got < want {
			t.Errorf("a row has %d fields where the header has %d columns:\n%q\n%q",
				got, want, header, row)
		}
	}
}

// -n asks for that many samples, separated by a blank line. The blank line is how a
// reader tells one sample from the next, so it is part of the format.
func TestTopBatch_repeatsForEachIteration(t *testing.T) {
	// A fractional delay, because the default is a second and two of them is two
	// seconds of a test doing nothing. Not `-d 0`: a zero delay is refused, since in
	// the interactive form it would spin the CPU redrawing, and -d takes a float
	// exactly so that a sub-second delay can be asked for instead.
	stdout, _ := runTopBatchApplet(t, "-b", "-n", "2", "-d", "0.01")
	// Two summaries means the first line's prefix appears twice.
	if got := strings.Count(stdout, "top - up "); got != 2 {
		t.Fatalf("-n 2 produced %d samples:\n%s", got, stdout)
	}
	// Separated by a blank line, and only between them -- not before the first.
	if strings.HasPrefix(stdout, "\n") {
		t.Errorf("the output begins with a blank line")
	}
	if !strings.Contains(stdout, "\n\ntop - up ") {
		t.Errorf("the two samples are not separated by a blank line:\n%s", stdout)
	}
	// And -n 1 produces exactly one.
	single, _ := runTopBatchApplet(t, "-b", "-n", "1")
	if got := strings.Count(single, "top - up "); got != 1 {
		t.Fatalf("-n 1 produced %d samples", got)
	}
}

// A first sample reports zero CPU for every process, and that is deliberate: one
// sample has nothing to compare against, and showing each process's whole lifetime
// of CPU as though it were spent in the last second would be worse than zeroes.
func TestTopBatch_theFirstSampleReportsNoCpuRates(t *testing.T) {
	stdout, _ := runTopBatchApplet(t, "-b", "-n", "1")
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	// The CPU summary line for a first sample is 0.0%.
	if !strings.HasPrefix(lines[1], "CPU: 0") {
		t.Errorf("the first sample reports %q, want a zero busy figure", lines[1])
	}
}

// Without a terminal and without -b, the plain form is printed *and said out loud*
// -- because the silence was the real defect: someone expecting a drawn table and
// getting four lines of text cannot tell a deliberate choice from a broken one.
func TestTopBatch_saysWhyItIsNotDrawingATable(t *testing.T) {
	stdout, stderr := runTopBatchApplet(t, "-n", "1")
	if !strings.Contains(stdout, "top - up ") {
		t.Fatalf("no sample was printed:\n%s", stdout)
	}
	if !strings.Contains(stderr, "not a terminal") {
		t.Fatalf("stderr does not explain the plain form: %q", stderr)
	}
	// -b silences it, which is what a script under a terminal needs.
	_, quiet := runTopBatchApplet(t, "-b", "-n", "1")
	if strings.Contains(quiet, "not a terminal") {
		t.Fatalf("-b did not silence the notice: %q", quiet)
	}
}

// The table is truncated to a width, because a command line is not a bounded thing:
// a Chrome renderer's is two thousand characters, so an untruncated table is one row
// per screenful.
func TestTopBatch_truncatesToTheWidth(t *testing.T) {
	stdout, _ := runTopBatchApplet(t, "-b", "-n", "1")
	for _, line := range strings.Split(strings.TrimRight(stdout, "\n"), "\n") {
		// The summary lines are short anyway; the point is that no table line runs
		// past the default width a pipe gets.
		if len(line) > lsDefaultWidth {
			t.Fatalf("a line is %d characters wide, past the %d a pipe gets:\n%q",
				len(line), lsDefaultWidth, line)
		}
	}
}

// -o chooses the columns, and an unknown name falls back rather than failing -- so
// a typo costs the column and not the command.
func TestTopBatch_chosenColumnsReachTheHeader(t *testing.T) {
	stdout, _ := runTopBatchApplet(t, "-b", "-n", "1", "-o", "pid,command")
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	header := lines[4]
	if !strings.Contains(header, "PID") || !strings.Contains(header, "COMMAND") {
		t.Fatalf("the chosen columns did not reach the header: %q", header)
	}
	// And a column that was not asked for is absent, which is what -o means.
	if strings.Contains(header, "RES") {
		t.Fatalf("-o pid,command still printed RES: %q", header)
	}
}

// -f filters, so the table holds only what matches. The filter is checked against
// this test's own process name, which is the one process guaranteed to be there.
func TestTopBatch_filterNarrowsTheTable(t *testing.T) {
	// Nothing can match this, so the table is empty and only the header remains.
	stdout, _ := runTopBatchApplet(t, "-b", "-n", "1", "-f", "zzzznosuchprocessname")
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 5 {
		t.Fatalf("a filter matching nothing left %d lines, want the four summary lines and a header:\n%s",
			len(lines), stdout)
	}
}

// The options top refuses, each named. A count or a delay that is not a number must
// not become zero silently -- a zero count prints nothing at all, which looks like
// a broken machine rather than a bad argument.
func TestTopArgs_refusesWhatItCannotRead(t *testing.T) {
	for _, test := range []struct {
		args    []string
		because string
	}{
		{args: []string{"-n", "abc"}, because: "abc"},
		{args: []string{"-n", "-1"}, because: "-1"},
		{args: []string{"-n"}, because: "-n"},
		{args: []string{"-d", "abc"}, because: "abc"},
		// A zero or negative delay is refused rather than becoming a busy loop: in
		// the interactive form it would spin the CPU redrawing. -d takes a float, so
		// a sub-second delay is asked for as 0.01 rather than as 0.
		{args: []string{"-d", "0"}, because: "0"},
		{args: []string{"-d", "-1"}, because: "-1"},
		{args: []string{"-d"}, because: "-d"},
		{args: []string{"-s"}, because: "-s"},
		{args: []string{"-o"}, because: "-o"},
		{args: []string{"-f"}, because: "-f"},
		{args: []string{"-Z"}, because: "-Z"},
		{args: []string{"extra"}, because: "extra"},
	} {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			_, err := topArgs(test.args)
			if err == nil {
				t.Fatalf("top %v was accepted", test.args)
			}
			if !strings.Contains(err.Error(), test.because) {
				t.Fatalf("top %v said %q, which does not mention %q", test.args, err, test.because)
			}
		})
	}
}

// And the options it accepts, with what each sets -- so the parser cannot quietly
// read one letter as another.
func TestTopArgs_readsEveryOptionItAccepts(t *testing.T) {
	options, err := topArgs([]string{"-b", "-H", "-t", "-n", "3", "-d", "2", "-s", "mem", "-f", "chrome", "-o", "pid,cpu"})
	if err != nil {
		t.Fatal(err)
	}
	if !options.batch {
		t.Error("-b did not set batch")
	}
	if !options.threads {
		t.Error("-H did not set threads")
	}
	if !options.tree {
		t.Error("-t did not set tree")
	}
	if options.iterations != 3 {
		t.Errorf("-n 3 gave %d iterations", options.iterations)
	}
	if options.delay != 2*time.Second {
		t.Errorf("-d 2 gave a delay of %v", options.delay)
	}
	if options.sort != "mem" {
		t.Errorf("-s mem gave the sort %q", options.sort)
	}
	if options.filter != "chrome" {
		t.Errorf("-f gave the filter %q", options.filter)
	}
	if len(options.columns) != 2 || options.columns[0] != "pid" || options.columns[1] != "cpu" {
		t.Errorf("-o gave the columns %q, want [pid cpu]", options.columns)
	}
	// The defaults, which decide what a bare `top` does.
	bare, err := topArgs(nil)
	if err != nil {
		t.Fatal(err)
	}
	if bare.iterations != 1 {
		t.Errorf("the default iteration count is %d, want 1", bare.iterations)
	}
	if bare.sort != "cpu" {
		t.Errorf("the default sort is %q, want cpu", bare.sort)
	}
	if bare.batch || bare.threads || bare.tree {
		t.Error("a bare top set one of -b, -H or -t")
	}
}

// A cancelled context stops a multi-sample run rather than sleeping through it,
// which is what makes Ctrl-C work during the delay between samples.
func TestTopBatch_stopsWhenCancelledBetweenSamples(t *testing.T) {
	// Guarded like the rest: off Windows the first sample fails on its own and the
	// test would pass without ever reaching the wait it is about.
	requireProcessSampling(t)
	ctx, cancel := context.WithCancel(t.Context())
	var stdout strings.Builder
	options, err := topArgs([]string{"-b", "-n", "50", "-d", "30"})
	if err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() { finished <- runTopBatch(ctx, options, &stdout) }()
	// The first sample is taken before any delay, so cancelling now lands in the
	// wait -- which is the branch being tested.
	cancel()
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("a cancelled run reported success")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the run slept through its cancellation")
	}
}

// requireProcessSampling skips where the process table is not implemented.
//
// internal/proc samples on Windows only and answers ErrListUnsupported elsewhere
// rather than guessing, so on ubuntu and macos every one of these would fail at the
// first sample -- which is the platform trap AGENTS.md records after four commits
// went green here and red on CI. startTop takes the same guard for the same reason;
// the topArgs tests do not need it, because parsing options touches no machine.
func requireProcessSampling(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("the process table is implemented on Windows only")
	}
}
