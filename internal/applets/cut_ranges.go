package applets

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

// cutRange is one range of cut's LIST, counted from 0 as busybox's cut_range is; an end of
// math.MaxInt is N-, to the end of the line.
type cutRange struct {
	start, end int
}

// cutComplement stands for GNU's --complement, which has no letter of its own; a control
// character no one types.
const cutComplement = "\x01"

// complementCutRanges is what ranges, in order and joined, leave out: GNU's --complement. When
// they leave nothing, a range that begins past any line selects nothing.
func complementCutRanges(ranges []cutRange) []cutRange {
	var complement []cutRange
	next := 0
	for _, r := range ranges {
		if r.start > next {
			complement = append(complement, cutRange{start: next, end: r.start - 1})
		}
		if r.end == math.MaxInt {
			next = math.MaxInt
			break
		}
		next = r.end + 1
	}
	if next != math.MaxInt {
		complement = append(complement, cutRange{start: next, end: math.MaxInt})
	}
	if len(complement) == 0 {
		complement = []cutRange{{start: math.MaxInt - 1, end: math.MaxInt - 1}}
	}
	return complement
}

// parseCutList is busybox's: N, N-, N-M and -M between commas, each N busybox's
// xatoi_positive, so `invalid number 'x'`, and a range that begins at 0 or ends before it
// begins `invalid range 3-1`. The ranges are sorted by where they begin unless -D, which
// keeps them as given, a range twice as well.
func parseCutList(list string, keepOrder bool) ([]cutRange, error) {
	if list == "" {
		return nil, errors.New("missing list of positions")
	}
	var ranges []cutRange
	for _, token := range strings.Split(list, ",") {
		if token == "" {
			return nil, fmt.Errorf("invalid range %s", list)
		}
		first, rest, dashed := strings.Cut(token, "-")
		if first == "" && rest == "" {
			return nil, fmt.Errorf("invalid range %s", token)
		}
		start := 0
		if first != "" {
			value, err := positiveNumber(first)
			if err != nil {
				return nil, err
			}
			start = value - 1
		}
		end := start
		shown := first
		if dashed {
			end, shown = math.MaxInt, rest
			if rest != "" {
				value, err := positiveNumber(rest)
				if err != nil {
					return nil, err
				}
				end = value - 1
			}
		}
		if start < 0 || end < start {
			return nil, fmt.Errorf("invalid range %s-%s", first, shown)
		}
		ranges = append(ranges, cutRange{start: start, end: end})
	}
	if keepOrder {
		return ranges, nil
	}
	sort.SliceStable(ranges, func(i, j int) bool { return ranges[i].start < ranges[j].start })
	// A range that begins inside the one before it is folded into it. busybox walks the two
	// apart and prints what they share twice: -f 1,1 prints a line with no tab and then a tab,
	// and a line that begins with two prints its empty second field as the first's. One that
	// only touches the one before stays apart, as -F joins two ranges by -O and keeps the
	// delimiters inside one.
	merged := ranges[:1]
	for _, r := range ranges[1:] {
		if last := &merged[len(merged)-1]; r.start <= last.end {
			last.end = max(last.end, r.end)
			continue
		}
		merged = append(merged, r)
	}
	return merged, nil
}
