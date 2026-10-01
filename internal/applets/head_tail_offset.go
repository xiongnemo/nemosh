package applets

import (
	"bufio"
	"io"
	"math"
	"strings"
)

// The sign in front of a count means something, and it was being thrown away.
//
//	tail -n +10 file    from line 10 to the end
//	head -n -2 file     everything except the last two lines
//
// `strconv.Atoi` accepts a leading `+`, so `-n +10` parsed as 10 and `tail` printed the last
// ten lines instead of starting at the tenth. Measured against busybox-w32 on a twelve-line
// file: it answers `j k l` and this answered `c` through `l`. A wrong answer with no
// diagnostic, which is the shape worth hunting. The `-N` form was at least loud -- it failed
// with `invalid count: -2` -- but busybox implements it and so does GNU.

// countSpec is a count together with what its sign asked for.
type countSpec struct {
	count int
	// bytes is -c rather than -n.
	bytes bool
	// fromStart is the `+N` form: begin at N rather than end there. Only tail has it;
	// GNU head accepts the `+` and ignores it, and so does this.
	fromStart bool
	// allButLast is the `-N` form given to head: everything except the final N.
	allButLast bool
}

// parseCountSpec reads a count operand, keeping the sign.
func parseCountSpec(text string, bytes bool) (countSpec, error) {
	spec := countSpec{bytes: bytes}
	digits := text
	switch {
	case strings.HasPrefix(text, "+"):
		spec.fromStart, digits = true, text[1:]
	case strings.HasPrefix(text, "-"):
		spec.allButLast, digits = true, text[1:]
	}
	// busybox's exact wording, single quotes included: `head -n2c` answers
	// `head: invalid number '2c'`. Measured 2026-08-22. This said
	// `invalid count: 2c`, which named the same problem in different words --
	// and a script matching on the reference's text would miss it.
	//
	// The digits rather than the whole operand, because that is what busybox
	// reports once a sign has been taken off: `head -n-x` answers
	// `invalid number 'x'`, not `'-x'`.
	value, err := headTailCount(digits)
	if err != nil {
		return countSpec{}, err
	}
	spec.count = value
	return spec, nil
}

// headTailCount is a count once its sign is off, as busybox's eat_num reads one with
// bkm_suffixes: decimal digits, and b, k or m after them for 512, 1024 or 1048576. A second
// sign, a blank or any other letter is an invalid number. `head -c 1k` was one.
func headTailCount(digits string) (int, error) {
	value, err := busyboxNumberBase(digits, 10, math.MaxInt, math.MaxInt, bkmSuffixes)
	return int(value), err
}

// copyLinesFromStart is `tail -n +N`: everything from line N onward, counting from 1.
//
// `+0` and `+1` both mean the whole input, which is what both references do -- there is no
// line zero to skip to. Each line keeps the ending it came with, as copyTail keeps it, so a
// last line with none is written with none; one was added, and a CRLF became an LF.
func copyLinesFromStart(stdout io.Writer, input io.Reader, count int) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 0, 64*1024), maxTextLine)
	scanner.Split(scanLineWithEnding)
	seen := 0
	for scanner.Scan() {
		seen++
		if seen < count {
			continue
		}
		if _, err := io.WriteString(stdout, scanner.Text()); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// copyBytesFromStart is `tail -c +N`: from byte N onward, counting from 1.
func copyBytesFromStart(stdout io.Writer, input io.Reader, count int) error {
	skip := count - 1
	if skip > 0 {
		if _, err := io.CopyN(io.Discard, input, int64(skip)); err != nil {
			// A short input leaves nothing to print, which is not a failure.
			return nil
		}
	}
	_, err := io.Copy(stdout, input)
	return err
}

// copyAllButLastLines is `head -n -N`: every line except the final N.
//
// The whole input has to be read before the first line can be printed, because which lines
// are "the last N" is not known until the end. A ring of N+1 lines is enough state for that:
// a line leaves the ring only once N further lines have arrived behind it, which proves it is
// not one of the last N.
func copyAllButLastLines(stdout io.Writer, input io.Reader, count int) error {
	if count == 0 {
		return copyLinesFromStart(stdout, input, 1)
	}
	// Each line keeps its ending, as copyLinesFromStart keeps it; a CRLF became an LF.
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 0, 64*1024), maxTextLine)
	scanner.Split(scanLineWithEnding)
	held := make([]string, 0, count+1)
	for scanner.Scan() {
		held = append(held, scanner.Text())
		if len(held) <= count {
			continue
		}
		if _, err := io.WriteString(stdout, held[0]); err != nil {
			return err
		}
		held = held[1:]
	}
	return scanner.Err()
}

// copyAllButLastBytes is `head -c -N`.
func copyAllButLastBytes(stdout io.Writer, input io.Reader, count int) error {
	data, err := io.ReadAll(input)
	if err != nil {
		return err
	}
	if len(data) <= count {
		return nil
	}
	_, err = stdout.Write(data[:len(data)-count])
	return err
}
