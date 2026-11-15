package main

import (
	"math/rand/v2"
	"testing"
	"time"
)

// roundsRunner answers head with the time for the round it is in, and base with a
// steady 10ms, so a test can say which rounds read as slower.
func roundsRunner(headByRound []time.Duration, pairsPerRound int) (runner, *int) {
	runs := 0
	return func(binary string, args []string) (time.Duration, error) {
		round := runs / (2 * pairsPerRound)
		runs++
		if binary == "head" {
			return headByRound[min(round, len(headByRound)-1)], nil
		}
		return 10 * time.Millisecond, nil
	}, &runs
}

func TestCompareScenario_believesASlowdownOnlyWhenEveryRoundSeesIt(t *testing.T) {
	ms := time.Millisecond
	limit := threshold{ratio: 1.10, floor: time.Millisecond}
	for _, test := range []struct {
		name        string
		headByRound []time.Duration
		wantSlower  bool
		wantRounds  int
	}{
		{name: "never slower", headByRound: []time.Duration{10 * ms}, wantSlower: false, wantRounds: 1},
		{name: "slower once, then not", headByRound: []time.Duration{15 * ms, 10 * ms}, wantSlower: false, wantRounds: 2},
		{name: "slower until the last round", headByRound: []time.Duration{15 * ms, 15 * ms, 10 * ms}, wantSlower: false, wantRounds: 3},
		{name: "slower in every round", headByRound: []time.Duration{15 * ms, 15 * ms, 15 * ms}, wantSlower: true, wantRounds: 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given: one warmup pair and three kept, confirmed twice
			run, _ := roundsRunner(test.headByRound, 1+3)
			s := scenario{name: "probe", limit: limit}

			// When
			rounds, isSlower, err := compareScenario(run, "base", "head", s, 1, 3, 2, rand.New(rand.NewPCG(1, 2)))

			// Then
			if err != nil {
				t.Fatalf("compareScenario: %v", err)
			}
			if isSlower != test.wantSlower || len(rounds) != test.wantRounds {
				t.Fatalf("slower = %v after %d rounds, want %v after %d: %+v",
					isSlower, len(rounds), test.wantSlower, test.wantRounds, rounds)
			}
		})
	}
}
