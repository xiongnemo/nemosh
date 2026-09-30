package applets

import (
	"sort"
	"strings"
	"time"
)

// sortLsEntries orders one directory's worth of entries.
//
// Before 2026-08-22 this was a bare name comparison and -t, -S and -r were all
// refused, which made `ls -ltr` -- about as well-worn a command as there is --
// fail on its options.
func sortLsEntries(items []lsEntry, options lsOptions) {
	sort.SliceStable(items, func(i, j int) bool {
		return lsEntryOrder(items[i], items[j], options) < 0
	})
}

// lsEntryOrder is busybox's sortcmp for one pair: directories first under
// --group-directories-first, then -S or -t, and what they leave tied by -v or else -X, and
// the name last of all.
//
// The name is the tie-break for every key, and it has to be: two files of the
// same size in the same second must come out in the same order on every run, or
// two listings of an unchanged directory would differ and a diff between them
// would mean nothing.
//
// -r reverses the whole order, the tie-break and the directories first with it, which is what
// makes `ls -tr` the exact reverse of `ls -t` rather than nearly it.
func lsEntryOrder(left, right lsEntry, options lsOptions) int {
	order := 0
	if options.dirsFirst {
		order = boolOrder(right.info.IsDir()) - boolOrder(left.info.IsDir())
	}
	if order == 0 {
		switch {
		case options.sortKey == lsSortBySize:
			order = compareInt64(right.info.Size(), left.info.Size())
		case options.sortKey == lsSortByTime:
			// In whole seconds, as busybox's time_t holds them: files made in one second
			// are ordered by name, where their nanoseconds put them in the order made.
			order = compareInt64(right.when(options).Unix(), left.when(options).Unix())
		case options.version:
			order = versionCompare(left.name, right.name)
		case options.extension:
			order = strings.Compare(lsExtension(left.name), lsExtension(right.name))
		}
	}
	if order == 0 {
		order = strings.Compare(left.name, right.name)
	}
	if options.reverse {
		return -order
	}
	return order
}

// when is the time -l shows and -t orders by: the modification time, or -c's or -u's.
func (e lsEntry) when(options lsOptions) time.Time {
	switch {
	case options.timeKey == lsChangeTime && e.recorded:
		return e.record.change
	case options.timeKey == lsAccessTime && e.recorded:
		return e.record.access
	}
	return e.info.ModTime()
}

// lsExtension is what -X orders by: a name from its first dot, as busybox's strchrnul finds
// it, so `x.tar.gz` sorts as `.tar.gz` and a name with no dot first of all.
func lsExtension(name string) string {
	if at := strings.IndexByte(name, '.'); at >= 0 {
		return name[at:]
	}
	return ""
}

func boolOrder(value bool) int {
	if value {
		return 1
	}
	return 0
}

func compareInt64(left, right int64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	}
	return 0
}

// versionCompare is glibc's strverscmp, which busybox's -v calls: a run of digits compares as a
// number, `a2` before `a10`, and a run that begins with 0 as a fraction, `a09` before `a1`.
func versionCompare(left, right string) int {
	const normal, integral, fraction, zeroes = 0, 3, 6, 9
	const compare, length = 2, 3
	next := [...]int{normal, integral, zeroes, normal, integral, integral, normal, fraction, fraction, normal, fraction, zeroes}
	result := [...]int{
		compare, compare, compare, compare, length, compare, compare, compare, compare,
		compare, -1, -1, 1, length, length, 1, length, length,
		compare, compare, compare, compare, compare, compare, compare, compare, compare,
		compare, 1, 1, -1, compare, compare, -1, compare, compare,
	}
	class := func(text string, at int) int {
		if at >= len(text) || text[at] < '0' || text[at] > '9' {
			return 0
		}
		if text[at] == '0' {
			return 2
		}
		return 1
	}
	byteAt := func(text string, at int) int {
		if at < len(text) {
			return int(text[at])
		}
		return 0
	}
	at := 0
	state := normal + class(left, 0)
	for byteAt(left, at) == byteAt(right, at) {
		if at >= len(left) {
			return 0
		}
		state = next[state]
		at++
		state += class(left, at)
	}
	diff := byteAt(left, at) - byteAt(right, at)
	switch kind := result[state*3+class(right, at)]; kind {
	case compare:
		return diff
	case length:
		// Two runs of digits that differ here: the longer is the larger, and of two as long
		// the first digit that differs decides.
		for probe := at + 1; ; probe++ {
			leftDigit, rightDigit := class(left, probe) != 0, class(right, probe) != 0
			switch {
			case !leftDigit && rightDigit:
				return -1
			case !leftDigit:
				return diff
			case !rightDigit:
				return 1
			}
		}
	default:
		return kind
	}
}
