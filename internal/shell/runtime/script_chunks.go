package runtime

import (
	"context"
	"errors"
	"maps"
	"strings"
)

// A text in bash mode is read as bash reads one: a command at a time, each read with the aliases
// in force once the one before it has run, and not read at all past an `exit`. What comes before a
// syntax error runs, the error ends the text where it is, status 2, and an installer's archive
// after `exit 0`, or code for another shell after `case $SH in bash) exit;; esac`, is never parsed.
// See docs/design/bash-mode-parsing.md.
//
// Reading it whole is the same thing done faster, the commands running in order either way, and
// is tried first, at the start and again wherever the text is read again; a command at a time is
// what a text that does not parse whole is read as from there on. The aliases a text was read
// with changing as it runs reads the rest of it again, from the next command; see runRead.
//
// A command at a time, the session's parse says whether the lines read so far end in a whole
// command, and is asked at one line, two, four and so on, rather than at every line: a session
// asks again from the first line of a command at each line, which for a three-thousand-line loop
// took six seconds. An error found that way narrows the question to the lines it took; see
// chunkWithin.

// readText is a text being read in bash mode, and the part of it read last.
type readText struct {
	lines []string
	first int // the source line of lines[0]
	// start and end are the lines read last: start where a command begins, end past the last.
	start, end int
	// reading is the aliases they were read with, and print its fingerprint; see alias_reading.go.
	reading map[string]string
	print   uint64
}

// runChunks runs source as bash mode reads a text, its first line numbered first, `$?` beginning
// as savedStatus: a script's as the top level runs it, and eval's text or a sourced file's,
// topLevel false, as executeRead does. It answers as they do, 0 when nothing ran, and the syntax
// error that ended it, if any.
func (r Runtime) runChunks(ctx context.Context, source string, first, savedStatus int, topLevel bool) (int, flowControl, error) {
	lines := strings.SplitAfter(source, "\n")
	if last := len(lines) - 1; last >= 0 && lines[last] == "" {
		lines = lines[:last]
	}
	status, ran, whole, parsed := savedStatus, false, true, 0
	for start := 0; start < len(lines); {
		text := &readText{lines: lines, first: first, start: start, end: len(lines), reading: maps.Clone(r.readingAliases()), print: r.aliasReading()}
		var prepared Script
		var err error
		if whole {
			parsed += len(lines) - start
			prepared, err = text.parse(start, len(lines))
			whole = err == nil
		}
		if !whole {
			if prepared, text.end, err = chunkWithin(text, 0, len(lines)-start, nil); err != nil {
				return status, flowNone, err
			}
		}
		r.sayParseWarnings(prepared)
		ran = ran || len(prepared.program) > 0
		var control flowControl
		var resume int
		status, control, resume = r.runRead(ctx, prepared.program, status, topLevel, text)
		if control != flowNone || r.lifecycle.exitSuppressed {
			return status, control, nil
		}
		if ctx.Err() != nil {
			return contextStatus(ctx), flowNone, nil
		}
		start = text.end
		if resume > 0 {
			// Read whole again while the lines parsed stay within a few times the text's: a file
			// defining alias after alias, each used further on, read the rest again for every one,
			// and fifty in an rc file of six hundred lines took 114 ms where it had taken 13.
			start = resume
			whole = whole && parsed+len(lines)-start <= 3*len(lines)
		}
	}
	if !ran {
		return 0, flowNone, nil
	}
	return status, flowNone, nil
}

// chunkWithin finds a whole command at the front of the lines from text.start, knowing that its
// first low lines are none and that its first high are the error highErr -- or, highErr nil, the
// end of the input. It asks at low+1, low+2, low+4 and on: a whole one is the answer, and an error
// narrows the search to the lines between it and the last that were not yet whole, asked in turn
// the same way. A whole command before the bad line is so always found, and a long one costs a few
// dozen parses rather than one for each of its lines. It answers the line past what it read.
func chunkWithin(text *readText, low, high int, highErr error) (Script, int, error) {
	start := text.start
	last := low
	for step := 1; low+step < high; step *= 2 {
		size := low + step
		prepared, err := text.parse(start, start+size)
		if err == nil {
			return prepared, start + size, nil
		}
		if !errors.Is(err, ErrIncompleteScript) {
			return chunkWithin(text, last, size, err)
		}
		last = size
	}
	if high-last > 1 {
		return chunkWithin(text, last, high, highErr)
	}
	if highErr != nil {
		return Script{}, start + high, highErr
	}
	// The end of the input, parsed as a script's end is: a here-document's warning and all.
	prepared, err := text.parse(start, start+high)
	return prepared, start + high, err
}

// parse parses lines[start:end] as numbered from where they are, with the aliases text was read
// with: as a session's lines while more input follows, and as a script's end at the end of it.
func (t *readText) parse(start, end int) (Script, error) {
	return parseReading(strings.Join(t.lines[start:end], ""), t.first+start, end < len(t.lines), t.reading)
}
