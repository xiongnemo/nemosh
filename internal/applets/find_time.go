package applets

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// findTime is -mtime -atime -ctime, counted in days, and -mmin -amin -cmin, in minutes. The
// file's age in whole seconds is held against count units, as busybox's time_cmp holds it:
// N is at least N units and short of N+1, +N at least N+1, and -N short of N. So -mtime 0 is
// changed in the last day, and -mtime +1 is at least two days old. A file stamped in the future
// has a negative age, which only -N is true of. -mtime clamped that age to 0, so `-mtime 0` was
// true of it, where busybox says false.
type findTime struct {
	// which is m, a or c: the modification, access or change time.
	which      byte
	comparison byte
	count      int64
	unit       int64
	now        time.Time
}

// timePredicate reads one of them. The change time is the creation time on Windows, which keeps
// no change time, as busybox-w32's stat reports it; see hostStatRecord.
func (p *findParser) timePredicate(operand string) (findNode, error) {
	value, err := p.argument(operand)
	if err != nil {
		return nil, err
	}
	comparison, digits := splitFindComparison(value)
	count, err := strconv.ParseInt(digits, 10, 64)
	if err != nil || count < 0 {
		return nil, fmt.Errorf("invalid number %q", value)
	}
	unit := int64(24 * 60 * 60)
	if strings.HasSuffix(operand, "min") {
		unit = 60
	}
	return findTime{which: operand[1], comparison: comparison, count: count, unit: unit, now: time.Now()}, nil
}

func (n findTime) eval(c findCandidate, _ *findRun) bool {
	stamp, ok := c.stamp(n.which)
	if !ok {
		return false
	}
	age, limit := n.now.Unix()-stamp.Unix(), n.count*n.unit
	switch n.comparison {
	case '+':
		return age >= limit+n.unit
	case '-':
		return age < limit
	}
	return age >= limit && age < limit+n.unit
}

// stamp is the candidate's modification, access or change time. A synthetic device entry has
// a modification time and no other.
func (c findCandidate) stamp(which byte) (time.Time, bool) {
	info, err := c.info()
	if err != nil {
		return time.Time{}, false
	}
	if which == 'm' {
		return info.ModTime(), true
	}
	if c.host == "" {
		return time.Time{}, false
	}
	record, err := hostStatRecord(c.host, false, 0)
	if err != nil {
		return time.Time{}, false
	}
	if which == 'a' {
		return record.access, true
	}
	return record.change, true
}
