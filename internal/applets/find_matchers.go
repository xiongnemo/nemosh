package applets

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// The tests themselves: one type per predicate, each answering for one entry.
//
// Split from find_predicate.go, which parses them, because the two answer
// different questions -- what an operand means, and whether an entry satisfies
// it -- and together they crossed the 250-line ceiling.

type findName struct {
	pattern string
	fold    bool
}

func (n findName) eval(c findCandidate, _ *findRun) bool {
	// The basename, never the path: busybox uses fnmatch without FNM_PATHNAME,
	// and a basename carries no separator for `*` to cross -- which is why
	// path.Match is right here and wrong for -path.
	subject := path.Base(filepath.ToSlash(c.display))
	pattern := n.pattern
	if n.fold {
		// Both sides are folded, so -iname A.TxT matches a.txt as well as
		// -iname *.TXT does.
		subject = strings.ToLower(subject)
		pattern = strings.ToLower(pattern)
	}
	matched, err := path.Match(pattern, subject)
	return err == nil && matched
}

type findPath struct {
	matcher *regexp.Regexp
	fold    bool
}

func (n findPath) eval(c findCandidate, _ *findRun) bool {
	subject := filepath.ToSlash(c.display)
	if n.fold {
		subject = strings.ToLower(subject)
	}
	return n.matcher.MatchString(subject)
}

type findType struct{ letter byte }

func (n findType) eval(c findCandidate, _ *findRun) bool {
	if c.entry == nil {
		return false
	}
	mode := c.entry.Type()
	switch n.letter {
	case 'f':
		return mode.IsRegular()
	case 'd':
		return mode.IsDir()
	case 'l':
		return mode&fs.ModeSymlink != 0
	case 'c':
		return mode&fs.ModeCharDevice != 0
	}
	return false
}

type findSize struct {
	comparison byte
	count      int64
	unit       int64
}

func (n findSize) eval(c findCandidate, _ *findRun) bool {
	info, err := c.info()
	if err != nil {
		return false
	}
	// Divided by the unit and rounded up, which POSIX states for -size and GNU
	// applies to every unit. busybox-w32 compares the raw byte count against
	// N*unit instead; the divergence is recorded in docs/support-matrix.md.
	units := (info.Size() + n.unit - 1) / n.unit
	return compareFindCount(n.comparison, units, n.count)
}

type findMtime struct {
	comparison byte
	days       int64
	now        time.Time
}

func (n findMtime) eval(c findCandidate, _ *findRun) bool {
	info, err := c.info()
	if err != nil {
		return false
	}
	// Whole 24-hour periods, truncated, so -mtime 0 is "changed today" and
	// -mtime +1 is "at least two days old".
	age := int64(n.now.Sub(info.ModTime()) / (24 * time.Hour))
	if age < 0 {
		age = 0
	}
	return compareFindCount(n.comparison, age, n.days)
}

type findNewer struct{ than time.Time }

func (n findNewer) eval(c findCandidate, _ *findRun) bool {
	info, err := c.info()
	if err != nil {
		return false
	}
	return info.ModTime().After(n.than)
}

// findEmpty is true for a zero-length file and for a directory with no entries,
// so the directory case needs a read rather than a stat.
type findEmpty struct{}

func (findEmpty) eval(c findCandidate, _ *findRun) bool {
	info, err := c.info()
	if err != nil {
		return false
	}
	if !info.IsDir() {
		return info.Size() == 0
	}
	if c.host == "" {
		// A synthetic directory, which cannot be read through the filesystem.
		// /dev has entries, so false is the honest answer rather than a guess.
		return false
	}
	entries, err := os.ReadDir(c.host)
	return err == nil && len(entries) == 0
}

func compareFindCount(comparison byte, actual, wanted int64) bool {
	switch comparison {
	case '+':
		return actual > wanted
	case '-':
		return actual < wanted
	}
	return actual == wanted
}
