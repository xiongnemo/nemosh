package applets

import "strings"

// The hold space and the multiline commands.
//
// These are what make sed more than a line filter, and every classic one-liner
// is built from them: `1!G;h;$p` reverses a file, `:a;N;$!ba;s/\n/ /g` joins one,
// `N;P;D` slides a two-line window along it.
//
// The hold space is a second buffer that survives between lines; the pattern
// space can hold more than one line once `N` has appended to it, which is why
// `P` and `D` exist to work on the first line of it alone.

// runSedHoldCommand performs h, H, g, G and x.
//
// A newline joins on append, which is what makes the hold space accumulate lines
// rather than concatenate them -- and why `H;${x;s/\n/,/g;p}` starts its answer
// with a comma: the hold space begins empty, so the first append leaves a leading
// newline. That is the reference behaviour, not an off-by-one.
func runSedHoldCommand(command *sedCommand, cycle *sedCycle) sedControl {
	switch command.action {
	case 'h':
		cycle.hold = cycle.line
	case 'H':
		cycle.hold = cycle.hold + "\n" + cycle.line
	case 'g':
		cycle.line = cycle.hold
	case 'G':
		cycle.line = cycle.line + "\n" + cycle.hold
	case 'x':
		cycle.line, cycle.hold = cycle.hold, cycle.line
	}
	return sedNext
}

// runSedNextCommand performs n and N, both of which read another line.
//
// At end of input each one ends the run, and the ordinary auto-print rules still
// apply -- which is why `sed 'N;s/\n/ /'` on three lines answers `a b` and then
// `c`: the third line found no partner and was printed as it stood.
func runSedNextCommand(command *sedCommand, cycle *sedCycle) (sedControl, error) {
	if command.action == 'n' {
		// n prints first, then replaces. The print is the automatic one brought
		// forward, so -n suppresses it exactly as it would at the end of the
		// script.
		if !cycle.quiet {
			if err := cycle.write(cycle.line); err != nil {
				return sedQuit, err
			}
		}
		line, ok, err := cycle.readNext()
		if err != nil {
			return sedQuit, err
		}
		if !ok {
			// No further input, and the pattern space has already been written.
			return sedDeleted, nil
		}
		cycle.line = line
		return sedNext, nil
	}
	// N appends, keeping what is already there.
	line, ok, err := cycle.readNext()
	if err != nil {
		return sedQuit, err
	}
	if !ok {
		// No partner for this line. The pattern space still stands and is written
		// as it is -- which is why `sed 'N;s/\n/ /'` over three lines answers
		// `a b` and then a bare `c`. It is printed here rather than left to the
		// end-of-cycle print, because ending the run has to skip that print for
		// `q`'s sake and the two cannot share one signal.
		if !cycle.quiet {
			if err := cycle.write(cycle.line); err != nil {
				return sedQuit, err
			}
		}
		return sedQuit, nil
	}
	cycle.line = cycle.line + "\n" + line
	return sedNext, nil
}

// runSedFirstLineCommand performs P and D, which act on the pattern space up to
// its first newline.
//
// `D` is the only command that restarts the script without reading: with a
// newline left in the pattern space it deletes through it and begins again, and
// otherwise it is `d`. That loop is what `N;P;D` is.
func runSedFirstLineCommand(command *sedCommand, cycle *sedCycle) (sedControl, error) {
	first, rest, multiline := strings.Cut(cycle.line, "\n")
	if command.action == 'P' {
		return sedNext, cycle.write(first)
	}
	if !multiline {
		return sedDeleted, nil
	}
	cycle.line = rest
	return sedRestart, nil
}
