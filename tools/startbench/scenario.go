package main

import (
	"math/rand/v2"
	"time"
)

// scenario is one command line, timed on both binaries.
type scenario struct {
	name  string
	args  []string
	limit threshold
}

// scenarios are what is timed. The first is the start alone -- process, Go
// runtime, every package's init. The second adds building a shell and running one
// applet in it. The third is two thousand commands, which is where a cost paid
// per command rather than per start shows: the applet dispatch, the lookup, an
// allocation on the way in.
func scenarios() []scenario {
	start := threshold{ratio: 1.10, floor: time.Millisecond}
	return []scenario{
		{name: "--version", args: []string{"--version"}, limit: start},
		{name: "-c true", args: []string{"-c", "true"}, limit: start},
		{
			name:  "2000 commands",
			args:  []string{"-c", "i=0; while [ $i -lt 2000 ]; do true; i=$((i+1)); done"},
			limit: threshold{ratio: 1.10, floor: 5 * time.Millisecond},
		},
	}
}

type roundResult struct {
	base, head time.Duration
	slower     bool
}

// compareScenario times one scenario, base against head, and says whether head is
// slower: slower in the first round and again in every one of confirm rounds after
// it. The Oils suite has the same rule for the same reason -- a difference is run
// again before either mode believes it -- and here it is what keeps one noisy
// round on a shared runner from reading as a regression.
func compareScenario(run runner, base, head string, s scenario, warmup, runs, confirm int, rng *rand.Rand) ([]roundResult, bool, error) {
	var rounds []roundResult
	for range confirm + 1 {
		baseTimes, headTimes, err := sample(run, base, head, s.args, warmup, runs, rng)
		if err != nil {
			return rounds, false, err
		}
		result := roundResult{base: median(baseTimes), head: median(headTimes)}
		result.slower = slower(result.base, result.head, s.limit)
		rounds = append(rounds, result)
		if !result.slower {
			return rounds, false, nil
		}
	}
	return rounds, true, nil
}
