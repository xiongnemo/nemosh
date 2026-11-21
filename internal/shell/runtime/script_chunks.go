package runtime

import (
	"context"
	"errors"
	"strings"
)

// A script in bash mode is read a command at a time when it does not parse whole, as bash
// reads every script: what comes before a syntax error runs, the error ends the script where it
// is, status 2, and text after an `exit` is never read -- an installer's archive after `exit 0`,
// or code for another shell after `case $SH in bash) exit;; esac`. A script that parses whole is
// run as it always was, which is the same thing done faster, the commands being run in order
// either way. See docs/design/bash-mode-parsing.md.
//
// The session's parse says whether the lines read so far end in a whole command, and is asked
// at one line, two, four and so on, rather than at every line: a session asks again from the
// first line of a command at each line, which for a three-thousand-line loop took six seconds.
// An error found that way narrows the question to the lines it took; see chunkWithin.

// runChunks runs source a command at a time, its first line numbered first, `$?` beginning as
// savedStatus. It answers as executeTopLevel does, and the syntax error that ended it, if one did.
func (r Runtime) runChunks(ctx context.Context, source string, first, savedStatus int) (int, flowControl, error) {
	lines := strings.SplitAfter(source, "\n")
	if last := len(lines) - 1; last >= 0 && lines[last] == "" {
		lines = lines[:last]
	}
	status := savedStatus
	for start := 0; start < len(lines); {
		prepared, end, err := r.nextChunk(lines, start, first)
		if err != nil {
			return status, flowNone, err
		}
		r.sayParseWarnings(prepared)
		var control flowControl
		status, control = r.executeStatements(ctx, prepared.program, status, true, true)
		if control != flowNone || r.lifecycle.exitSuppressed {
			return status, control, nil
		}
		if ctx.Err() != nil {
			return contextStatus(ctx), flowNone, nil
		}
		start = end
	}
	return status, flowNone, nil
}

// nextChunk parses the whole command or commands that begin at line start, and answers the line
// after them; see chunkWithin.
func (r Runtime) nextChunk(lines []string, start, first int) (Script, int, error) {
	return chunkWithin(lines, start, first, 0, len(lines)-start, nil)
}

// chunkWithin finds a whole command at the front of lines[start:], knowing that its first low
// lines are none and that its first high are the error highErr -- or, highErr nil, the end of the
// input. It asks at low+1, low+2, low+4 and on: a whole one is the answer, and an error narrows
// the search to the lines between it and the last that were not yet whole, asked in turn the same
// way. A whole command before the bad line is so always found, and a long one costs a few dozen
// parses rather than one for each of its lines.
func chunkWithin(lines []string, start, first, low, high int, highErr error) (Script, int, error) {
	last := low
	for step := 1; low+step < high; step *= 2 {
		size := low + step
		prepared, err := parseChunk(lines, start, start+size, first)
		if err == nil {
			return prepared, start + size, nil
		}
		if !errors.Is(err, ErrIncompleteScript) {
			return chunkWithin(lines, start, first, last, size, err)
		}
		last = size
	}
	if high-last > 1 {
		return chunkWithin(lines, start, first, last, high, highErr)
	}
	if highErr != nil {
		return Script{}, start + high, highErr
	}
	// The end of the input, parsed as a script's end is: a here-document's warning and all.
	prepared, err := parseChunk(lines, start, start+high, first)
	return prepared, start + high, err
}

// parseChunk parses lines[start:end] as numbered from first: as a session's lines while more
// input follows, and as a script's end at the end of the input.
func parseChunk(lines []string, start, end, first int) (Script, error) {
	text := strings.Join(lines[start:end], "")
	if end >= len(lines) {
		return parseScriptAt(text, first+start)
	}
	return parseSessionAt(text, first+start)
}
