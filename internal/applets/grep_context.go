package applets

import (
	"fmt"
	"io"
)

// Context lines: -A, -B and -C.
//
// -A is easy and -B is the reason this needs state. Whether a line is context is
// not known when it is read -- it becomes context only once a *later* line
// matches -- so the last N non-printed lines have to be held. A line that was
// already printed never enters the ring, which is what keeps overlapping groups
// from printing anything twice.

// grepPending is one line held back in case something below it matches.
type grepPending struct {
	number int
	text   string
}

// grepRing holds at most limit lines, dropping the oldest.
type grepRing struct {
	held  []grepPending
	limit int
}

func (r *grepRing) push(number int, text string) {
	if r.limit == 0 {
		return
	}
	r.held = append(r.held, grepPending{number: number, text: text})
	if len(r.held) > r.limit {
		r.held = r.held[1:]
	}
}

func (r *grepRing) drain() []grepPending {
	held := r.held
	r.held = nil
	return held
}

// grepPrinter writes the output, and carries the state a separator needs.
//
// That state outlives one file on purpose: `--` appears between the last group of
// one file and the first of the next, which was measured -- so a printer scoped
// to a single file could not produce it.
type grepPrinter struct {
	stdout io.Writer
	flags  grepFlags
	// wrote is whether anything has been written at all, so the first group is
	// not preceded by a separator.
	wrote      bool
	lastName   string
	lastNumber int
}

// contextRequested reports whether any context was asked for.
//
// The separator depends on it rather than on the groups being adjacent: busybox
// prints no `--` at all under `-A0`, even between groups several lines apart.
// GNU does print one there; busybox is the reference this follows.
func (p *grepPrinter) contextRequested() bool {
	return p.flags.afterContext > 0 || p.flags.beforeContext > 0
}

// needsSeparator answers for the line about to be written. A different file is
// always a break; the same file breaks when a line was skipped.
func (p *grepPrinter) needsSeparator(name string, number int) bool {
	if !p.contextRequested() || !p.wrote {
		return false
	}
	if name != p.lastName {
		return true
	}
	return number > p.lastNumber+1
}

// emit writes one line, matching or context.
//
// A match is separated from its prefixes by a colon and a context line by a
// dash, for both the filename and the line number: `g.txt:2:M1` against
// `g.txt-3-l3`. That is how a reader tells the two apart, and it is what both
// references do.
func (p *grepPrinter) emit(name string, number int, text string, isMatch, withNames bool) error {
	if p.needsSeparator(name, number) {
		if _, err := fmt.Fprintln(p.stdout, "--"); err != nil {
			return err
		}
	}
	separator := ":"
	if !isMatch {
		separator = "-"
	}
	prefix := ""
	if withNames {
		prefix = name + separator
	}
	if p.flags.lineNumber {
		prefix += fmt.Sprintf("%d%s", number, separator)
	}
	if _, err := fmt.Fprintf(p.stdout, "%s%s\n", prefix, text); err != nil {
		return err
	}
	p.wrote, p.lastName, p.lastNumber = true, name, number
	return nil
}

// flushBefore writes the held lines as leading context.
func (p *grepPrinter) flushBefore(ring *grepRing, name string, withNames bool) error {
	for _, pending := range ring.drain() {
		if err := p.emit(name, pending.number, pending.text, false, withNames); err != nil {
			return err
		}
	}
	return nil
}

// reportsPerLine is whether this run prints lines at all. -c, -l, -L and -q
// answer per file instead, and context does not apply to them -- `grep -c -A1`
// counts matches and ignores the context entirely, which was measured.
func (f grepFlags) reportsPerLine() bool {
	return !f.countOnly && !f.filesOnly && !f.withoutMatch && !f.quiet
}
