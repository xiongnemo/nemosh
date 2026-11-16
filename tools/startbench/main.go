// Command startbench compares two nemosh binaries on the two things a start costs:
// the allocations package init makes, and the time a few command lines take. It
// reports a third, the commit charge of a session waiting for input, and holds it
// to a ceiling as it holds init: a number, the same from run to run.
//
// Allocations are a count, the same on every run of one binary, so they are held
// to a ceiling outright. Time is not -- five consecutive starts on an idle machine
// measured 43, 45, 42, 26 and 43 ms -- so it is never compared against a number:
// head is timed against base on the same machine in the same minute, the two
// interleaved, and a slowdown is believed only when it is past a ratio and a floor
// in every round. See docs/design/startup-and-footprint.md.
//
//	go run ./tools/startbench -base old.exe -head new.exe -init-ceiling 2750 -self-check
package main

import (
	"flag"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"time"
)

type options struct {
	base, head            string
	warmup, runs, confirm int
	initCeiling           int
	commitCeiling         int // MiB
	enforce, selfCheck    bool
}

func main() {
	var o options
	// Named nemosh-something, both: the binary answers to the name it was started by, so
	// one saved as head.exe is the head applet and fails every run.
	flag.StringVar(&o.base, "base", "", "the nemosh binary to compare against; with none, only package init is checked")
	flag.StringVar(&o.head, "head", "", "the nemosh binary under test")
	flag.IntVar(&o.warmup, "warmup", 5, "pairs run and discarded before each round")
	flag.IntVar(&o.runs, "runs", 40, "pairs kept in each round")
	flag.IntVar(&o.confirm, "confirm", 2, "rounds a slower scenario is run again before it is believed")
	flag.IntVar(&o.initCeiling, "init-ceiling", 0, "fail when head's package init allocates more than this; 0 for none")
	flag.IntVar(&o.commitCeiling, "commit-ceiling", 0, "fail when a session of head's commits more than this many MiB; 0 for none")
	flag.BoolVar(&o.enforce, "enforce", false, "fail on a confirmed slowdown, rather than saying it would")
	flag.BoolVar(&o.selfCheck, "self-check", false, "also time head against itself, which must never read as slower")
	flag.Parse()
	if o.head == "" {
		fmt.Fprintln(os.Stderr, "startbench: -head is required")
		os.Exit(2)
	}
	os.Exit(startbench(o, os.Stdout))
}

func startbench(o options, out io.Writer) int {
	status := 0
	if !checkInit(o, out) {
		status = 1
	}
	if !reportCommit(o, out) {
		status = 1
	}
	if o.base == "" {
		return status
	}
	// A fixed seed: the order is still mixed within a run, and the same from one run
	// to the next, so two runs differ only in what the machine did.
	rng := rand.New(rand.NewPCG(1, 2))
	pairs := []struct{ label, base string }{{"head against base", o.base}}
	if o.selfCheck {
		pairs = append(pairs, struct{ label, base string }{"head against itself", o.head})
	}
	for _, pair := range pairs {
		fmt.Fprintf(out, "time, %s (median of %d, %d confirming rounds):\n", pair.label, o.runs, o.confirm)
		for _, s := range scenarios() {
			rounds, isSlower, err := compareScenario(timeRun, pair.base, o.head, s, o.warmup, o.runs, o.confirm, rng)
			report(out, s, rounds)
			switch {
			case err != nil:
				fmt.Fprintf(out, "  %s: %v\n", s.name, err)
				status = 1
			case !isSlower:
			case pair.base == o.head:
				fmt.Fprintf(out, "  FALSE ALARM: %s read as slower than itself; the threshold is inside the noise\n", s.name)
			case o.enforce:
				fmt.Fprintf(out, "  SLOWER: %s, in every round\n", s.name)
				status = 1
			default:
				fmt.Fprintf(out, "  WOULD FAIL: %s is slower in every round (reported, not failed, without -enforce)\n", s.name)
			}
		}
	}
	return status
}

func report(out io.Writer, s scenario, rounds []roundResult) {
	for index, round := range rounds {
		name := s.name
		if index > 0 {
			name = "  again"
		}
		verdict := "ok"
		if round.slower {
			verdict = "slower"
		}
		change := 100 * (float64(round.head)/float64(round.base) - 1)
		fmt.Fprintf(out, "  %-14s base %7.2f ms   head %7.2f ms   %+6.1f%%   %s\n",
			name, milliseconds(round.base), milliseconds(round.head), change, verdict)
	}
}

func milliseconds(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

// checkInit reports head's package init, against base's when there is one, and
// holds it to the ceiling.
func checkInit(o options, out io.Writer) bool {
	head, err := traceInit(o.head)
	if err != nil {
		fmt.Fprintln(out, "package init:", err)
		return false
	}
	line := fmt.Sprintf("package init: head %d allocations", head.total())
	var base initTrace
	if o.base != "" {
		if base, err = traceInit(o.base); err != nil {
			fmt.Fprintln(out, "package init:", err)
			return false
		}
		line += fmt.Sprintf(", base %d (%+d)", base.total(), head.total()-base.total())
	}
	if o.initCeiling > 0 {
		line += fmt.Sprintf(", ceiling %d", o.initCeiling)
	}
	fmt.Fprintln(out, line)
	listPackages(out, "grew", growth(base, head))
	listPackages(out, "largest", growth(nil, head))
	if o.initCeiling > 0 && head.total() > o.initCeiling {
		fmt.Fprintln(out, "package init grew past the ceiling: something new is doing work before main")
		return false
	}
	return true
}

func listPackages(out io.Writer, label string, packages []packageGrowth) {
	format := "  %-8s %-48s %5d\n"
	if label == "grew" {
		format = "  %-8s %-48s %+5d\n"
	}
	for _, p := range packages[:min(5, len(packages))] {
		fmt.Fprintf(out, format, label, p.name, p.by)
	}
}
