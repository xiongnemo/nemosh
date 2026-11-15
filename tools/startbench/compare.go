package main

import (
	"slices"
	"time"
)

// threshold is how much slower head has to be than base to count, and it takes
// both: a ratio alone trips on a fraction of a millisecond when the whole run is
// five, and a floor alone ignores a long run getting a tenth slower.
type threshold struct {
	ratio float64
	floor time.Duration
}

// median is the middle sample, or the mean of the middle two. The median rather
// than the mean because a shared runner's noise only ever adds -- a descheduled
// process, a scan of a new file -- and one such run moves a mean a long way.
func median(samples []time.Duration) time.Duration {
	sorted := slices.Clone(samples)
	slices.Sort(sorted)
	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[middle]
	}
	return (sorted[middle-1] + sorted[middle]) / 2
}

func slower(base, head time.Duration, limit threshold) bool {
	return head-base > limit.floor && float64(head) > float64(base)*limit.ratio
}
