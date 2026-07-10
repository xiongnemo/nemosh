package applets

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type cutRange struct {
	start int
	end   int
}

func parseCutRanges(list string) ([]cutRange, error) {
	if list == "" {
		return nil, fmt.Errorf("cut: missing list of positions")
	}
	if strings.Contains(list, ",,") || strings.HasPrefix(list, ",") || strings.HasSuffix(list, ",") {
		return nil, fmt.Errorf("cut: invalid range %s", list)
	}
	parts := strings.Split(list, ",")
	ranges := make([]cutRange, 0, len(parts))
	for _, part := range parts {
		rangePart, err := parseCutRangePart(part)
		if err != nil {
			return nil, fmt.Errorf("cut: invalid range %s", part)
		}
		ranges = append(ranges, rangePart)
	}
	return mergeCutRanges(ranges), nil
}

func parseCutRangePart(part string) (cutRange, error) {
	if part == "" || part == "-" {
		return cutRange{}, errInvalidCutRange
	}
	if strings.HasPrefix(part, "-") {
		end, err := parseCutPosition(part[1:])
		if err != nil {
			return cutRange{}, err
		}
		return cutRange{start: 1, end: end}, nil
	}
	if prefix, ok := strings.CutSuffix(part, "-"); ok {
		start, err := parseCutPosition(prefix)
		if err != nil {
			return cutRange{}, err
		}
		return cutRange{start: start}, nil
	}
	if strings.Contains(part, "-") {
		bounds := strings.Split(part, "-")
		if len(bounds) != 2 {
			return cutRange{}, errInvalidCutRange
		}
		start, startErr := parseCutPosition(bounds[0])
		end, endErr := parseCutPosition(bounds[1])
		if startErr != nil || endErr != nil || start > end {
			return cutRange{}, errInvalidCutRange
		}
		return cutRange{start: start, end: end}, nil
	}
	position, err := parseCutPosition(part)
	if err != nil {
		return cutRange{}, err
	}
	return cutRange{start: position, end: position}, nil
}

var errInvalidCutRange = fmt.Errorf("invalid cut range")

func parseCutPosition(raw string) (int, error) {
	if strings.HasPrefix(raw, "+") {
		return 0, errInvalidCutRange
	}
	position, err := strconv.Atoi(raw)
	if err != nil || position <= 0 {
		return 0, errInvalidCutRange
	}
	return position, nil
}

func mergeCutRanges(ranges []cutRange) []cutRange {
	sort.Slice(ranges, func(left int, right int) bool {
		return ranges[left].start < ranges[right].start
	})
	merged := make([]cutRange, 0, len(ranges))
	for _, item := range ranges {
		if len(merged) == 0 {
			merged = append(merged, item)
			continue
		}
		last := &merged[len(merged)-1]
		if last.end == 0 {
			continue
		}
		if item.start > last.end+1 {
			merged = append(merged, item)
			continue
		}
		if item.end == 0 || item.end > last.end {
			last.end = item.end
		}
	}
	return merged
}

func cutRangeContains(ranges []cutRange, position int) bool {
	for _, item := range ranges {
		if position < item.start {
			return false
		}
		if item.end == 0 || position <= item.end {
			return true
		}
	}
	return false
}
