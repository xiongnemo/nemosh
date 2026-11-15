package main

import (
	"errors"
	"math/rand/v2"
	"slices"
	"testing"
	"time"
)

func TestSample_alternatesTheBinariesAndDiscardsTheWarmup(t *testing.T) {
	// Given: a runner that records the order it was asked in, and answers each
	// binary with a time of its own
	var order []string
	run := func(binary string, args []string) (time.Duration, error) {
		order = append(order, binary)
		if binary == "base" {
			return time.Millisecond, nil
		}
		return 2 * time.Millisecond, nil
	}

	// When
	baseTimes, headTimes, err := sample(run, "base", "head", []string{"--version"}, 2, 5, rand.New(rand.NewPCG(1, 2)))

	// Then: every pair has one of each, warmup included
	if err != nil {
		t.Fatalf("sample: %v", err)
	}
	if len(order) != 2*(2+5) {
		t.Fatalf("ran %d times, want %d: %v", len(order), 2*(2+5), order)
	}
	for pair := 0; pair < len(order); pair += 2 {
		got := []string{order[pair], order[pair+1]}
		slices.Sort(got)
		if got[0] != "base" || got[1] != "head" {
			t.Fatalf("pair %d ran %v, want one of each", pair/2, order[pair:pair+2])
		}
	}
	// and the warmup is not among the samples, each kept with its own binary
	if len(baseTimes) != 5 || len(headTimes) != 5 {
		t.Fatalf("kept %d and %d samples, want 5 each", len(baseTimes), len(headTimes))
	}
	for index := range baseTimes {
		if baseTimes[index] != time.Millisecond || headTimes[index] != 2*time.Millisecond {
			t.Fatalf("samples were crossed: base %v, head %v", baseTimes, headTimes)
		}
	}
}

func TestSample_ordersThePairsBothWays(t *testing.T) {
	// Given
	var firsts []string
	calls := 0
	run := func(binary string, args []string) (time.Duration, error) {
		if calls%2 == 0 {
			firsts = append(firsts, binary)
		}
		calls++
		return time.Millisecond, nil
	}

	// When
	if _, _, err := sample(run, "base", "head", nil, 0, 40, rand.New(rand.NewPCG(1, 2))); err != nil {
		t.Fatalf("sample: %v", err)
	}

	// Then: neither binary always goes first, which would charge it whatever
	// running first costs
	if !slices.Contains(firsts, "base") || !slices.Contains(firsts, "head") {
		t.Fatalf("one binary always went first: %v", firsts)
	}
}

func TestSample_keepsBothSamplesWhenBothSidesAreOneBinary(t *testing.T) {
	// Given: the self-check, which passes one path as both; each run answers a
	// different time, so a collision between the two would show as a lost one
	calls := 0
	run := func(binary string, args []string) (time.Duration, error) {
		calls++
		return time.Duration(calls), nil
	}

	// When
	baseTimes, headTimes, err := sample(run, "same", "same", nil, 0, 10, rand.New(rand.NewPCG(1, 2)))

	// Then: every one of the twenty runs is kept, once
	if err != nil {
		t.Fatalf("sample: %v", err)
	}
	seen := map[time.Duration]bool{}
	for _, elapsed := range append(slices.Clone(baseTimes), headTimes...) {
		seen[elapsed] = true
	}
	if len(seen) != 20 {
		t.Fatalf("kept %d distinct runs of 20: base %v, head %v", len(seen), baseTimes, headTimes)
	}
}

func TestSample_stopsAtAFailedRun(t *testing.T) {
	// Given
	failure := errors.New("exit status 2")
	run := func(binary string, args []string) (time.Duration, error) {
		return 0, failure
	}

	// When
	_, _, err := sample(run, "base", "head", nil, 0, 3, rand.New(rand.NewPCG(1, 2)))

	// Then
	if !errors.Is(err, failure) {
		t.Fatalf("sample error = %v, want %v", err, failure)
	}
}
