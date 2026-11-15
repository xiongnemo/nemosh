package main

import (
	"cmp"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
)

// initTrace is the allocations each package's init made in one run, by package, as
// GODEBUG=inittrace=1 reports them. Allocations rather than the clock: the clock in
// that trace moves in half-millisecond steps on Windows and reads 0 for nearly
// every package, while the count is the same on every run of the same binary.
type initTrace map[string]int

type packageGrowth struct {
	name string
	by   int
}

// parseInitTrace reads the trace's lines, which look like
//
//	init github.com/gdamore/encoding @0.99 ms, 1.0 ms clock, 66624 bytes, 1103 allocs
//
// and passes over everything else the run wrote.
func parseInitTrace(output string) initTrace {
	trace := initTrace{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[0] != "init" || fields[len(fields)-1] != "allocs" {
			continue
		}
		allocs, err := strconv.Atoi(fields[len(fields)-2])
		if err != nil {
			continue
		}
		trace[fields[1]] += allocs
	}
	return trace
}

func (t initTrace) total() int {
	sum := 0
	for _, allocs := range t {
		sum += allocs
	}
	return sum
}

// growth is the packages whose init allocates more in head than in base, the most
// first: where to look when the total moved.
func growth(base, head initTrace) []packageGrowth {
	var grown []packageGrowth
	for name, allocs := range head {
		if by := allocs - base[name]; by > 0 {
			grown = append(grown, packageGrowth{name: name, by: by})
		}
	}
	slices.SortFunc(grown, func(a, b packageGrowth) int {
		return cmp.Or(cmp.Compare(b.by, a.by), cmp.Compare(a.name, b.name))
	})
	return grown
}

// traceInit runs binary --version with the trace on. A run that traced nothing is an
// error rather than a total of zero, so the check cannot pass by measuring nothing.
func traceInit(binary string) (initTrace, error) {
	command := exec.Command(binary, "--version")
	command.Env = append(os.Environ(), "GODEBUG=inittrace=1")
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s --version: %w", binary, err)
	}
	trace := parseInitTrace(string(output))
	if len(trace) == 0 {
		return nil, fmt.Errorf("%s printed no inittrace; the check cannot report", binary)
	}
	return trace, nil
}
