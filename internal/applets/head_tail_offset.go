package applets

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
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
	value, err := strconv.Atoi(digits)
	if err != nil || value < 0 || strings.HasPrefix(digits, "+") || strings.HasPrefix(digits, "-") {
		return countSpec{}, fmt.Errorf("invalid count: %s", text)
	}
	spec.count = value
	return spec, nil
}

// copyLinesFromStart is `tail -n +N`: everything from line N onward, counting from 1.
//
// `+0` and `+1` both mean the whole input, which is what both references do -- there is no
// line zero to skip to.
func copyLinesFromStart(stdout io.Writer, input io.Reader, count int) error {
	scanner := bufio.NewScanner(input)
	seen := 0
	for scanner.Scan() {
		seen++
		if seen < count {
			continue
		}
		if _, err := fmt.Fprintln(stdout, scanner.Text()); err != nil {
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
	scanner := bufio.NewScanner(input)
	held := make([]string, 0, count+1)
	for scanner.Scan() {
		held = append(held, scanner.Text())
		if len(held) <= count {
			continue
		}
		if _, err := fmt.Fprintln(stdout, held[0]); err != nil {
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
