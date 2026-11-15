package main

import (
	"fmt"
	"math/rand/v2"
	"os/exec"
	"time"
)

// runner runs a binary once and says how long it took.
type runner func(binary string, args []string) (time.Duration, error)

// timeRun is the real runner: wall time from start to exit, output discarded. A
// failed run is an error, since a time for a command that did not do its work
// measures something else.
func timeRun(binary string, args []string) (time.Duration, error) {
	command := exec.Command(binary, args...)
	start := time.Now()
	err := command.Run()
	elapsed := time.Since(start)
	if err != nil {
		return 0, fmt.Errorf("%s %q: %w", binary, args, err)
	}
	return elapsed, nil
}

// sample runs base and head in pairs, warmup pairs first and then runs pairs that
// are kept, and returns each binary's times. The two are interleaved so whatever
// the machine is doing meanwhile lands on both alike: a runner that slows down
// halfway through slows both halves of every pair, where running one binary's
// forty and then the other's would charge it all to whichever went second. Which
// goes first is chosen afresh for each pair, for the same reason.
//
// The warmup is there for the first run of a new file, which is the slowest by far
// on Windows: Defender scans it before it is allowed to start.
func sample(run runner, base, head string, args []string, warmup, runs int, rng *rand.Rand) (baseTimes, headTimes []time.Duration, err error) {
	//
	// The times are kept by position in the pair, not by path: the self-check passes
	// the same binary as both.
	for pair := range warmup + runs {
		headFirst := rng.IntN(2) == 1
		order := [2]string{base, head}
		if headFirst {
			order = [2]string{head, base}
		}
		var times [2]time.Duration
		for index, binary := range order {
			elapsed, err := run(binary, args)
			if err != nil {
				return nil, nil, err
			}
			times[index] = elapsed
		}
		if headFirst {
			times[0], times[1] = times[1], times[0]
		}
		if pair >= warmup {
			baseTimes = append(baseTimes, times[0])
			headTimes = append(headTimes, times[1])
		}
	}
	return baseTimes, headTimes, nil
}
