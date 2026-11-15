package main

import (
	"testing"
	"time"
)

func TestMedian_isTheMiddleSampleOrTheMeanOfTheTwo(t *testing.T) {
	ms := time.Millisecond
	for _, test := range []struct {
		samples []time.Duration
		want    time.Duration
	}{
		{samples: []time.Duration{9 * ms, 1 * ms, 5 * ms}, want: 5 * ms},
		{samples: []time.Duration{4 * ms, 2 * ms, 8 * ms, 6 * ms}, want: 5 * ms},
		{samples: []time.Duration{7 * ms}, want: 7 * ms},
	} {
		if got := median(test.samples); got != test.want {
			t.Errorf("median(%v) = %v, want %v", test.samples, got, test.want)
		}
	}
}

func TestMedian_leavesTheSamplesInTheOrderTheyWereTaken(t *testing.T) {
	// Given
	samples := []time.Duration{3, 1, 2}

	// When
	median(samples)

	// Then
	if samples[0] != 3 || samples[1] != 1 || samples[2] != 2 {
		t.Fatalf("median reordered its argument: %v", samples)
	}
}

func TestSlower_needsBothTheRatioAndTheFloor(t *testing.T) {
	ms := time.Millisecond
	limit := threshold{ratio: 1.10, floor: 1 * ms}
	for _, test := range []struct {
		name       string
		base, head time.Duration
		want       bool
	}{
		{name: "the same", base: 10 * ms, head: 10 * ms, want: false},
		{name: "faster", base: 10 * ms, head: 8 * ms, want: false},
		{name: "past the ratio, under the floor", base: 5 * ms, head: 5*ms + 900*time.Microsecond, want: false},
		{name: "past the floor, under the ratio", base: 30 * ms, head: 32 * ms, want: false},
		{name: "past both", base: 10 * ms, head: 12 * ms, want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := slower(test.base, test.head, limit); got != test.want {
				t.Fatalf("slower(%v, %v) = %v, want %v", test.base, test.head, got, test.want)
			}
		})
	}
}
