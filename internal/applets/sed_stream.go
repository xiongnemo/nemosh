package applets

import (
	"bufio"
	"io"
)

// Several files are ONE stream.
//
// Measured against busybox-w32 on 2026-08-22: `sed -n '3p' f1.txt f2.txt`
// answers the third line *overall*, and `$` is the last line of the last file.
// So sed cannot loop over its operands applying the script to each -- which is
// what it did. That did not matter while sed had no addresses, and would have
// been silently wrong the moment it had.
//
// `$` also needs to know whether the current line is the last one, which means
// one line of lookahead. One line, not the whole input: sed is a filter, and
// reading a log into memory to find out where it ends is not what a filter does.

// sedStream yields lines across a sequence of inputs, telling the caller when it
// has reached the last one.
type sedStream struct {
	openers []func() (io.ReadCloser, error)
	next    int
	current io.ReadCloser
	scanner *bufio.Scanner
	// head is the line to yield, read one ahead so that isLast can be answered.
	head    string
	hasHead bool
	primed  bool
	// headEnded is whether the buffered head line had a terminator. It travels out
	// with the line, because the stream reads one ahead -- so asking the stream
	// after the fact answers about the *lookahead* and not about the line just
	// emitted.
	headEnded bool
	// onOpenError is how an unreadable operand is reported. busybox warns and
	// carries on with status 1 (editors/sed.c:1061), so the stream keeps going.
	onOpenError func(error)
}

// Next answers the line, whether that line was terminated in the input, and
// whether there was a line at all.
//
// The ending comes out *with* the line rather than being asked for afterwards: the
// stream reads one ahead so that the `$` address can be answered, so by the time a
// caller could ask, the buffered ending belongs to the next line.
func (s *sedStream) Next() (string, bool, bool, error) {
	if !s.primed {
		if err := s.fill(); err != nil {
			return "", false, false, err
		}
		s.primed = true
	}
	if !s.hasHead {
		return "", false, false, nil
	}
	line, ended := s.head, s.headEnded
	if err := s.fill(); err != nil {
		return "", false, false, err
	}
	return line, ended, true, nil
}

// fill reads the next line into head, crossing into the following input when the
// current one runs out.
func (s *sedStream) fill() error {
	for {
		if s.scanner != nil {
			if s.scanner.Scan() {
				// The token keeps its terminator, so the line is split off it here
				// and the terminator remembered. sed follows busybox in dropping a
				// carriage return -- measured, both turn a CRLF file into LF -- so
				// only
				// whether there *was* an ending is kept, not which one.
				line, ending := splitLineEnding(s.scanner.Text())
				s.head, s.hasHead, s.headEnded = line, true, ending != ""
				return nil
			}
			if err := s.scanner.Err(); err != nil {
				return err
			}
			if err := s.current.Close(); err != nil {
				return err
			}
			s.scanner, s.current = nil, nil
		}
		if s.next >= len(s.openers) {
			s.hasHead = false
			return nil
		}
		opener := s.openers[s.next]
		s.next++
		input, err := opener()
		if err != nil {
			s.onOpenError(err)
			continue
		}
		s.current = input
		s.scanner = bufio.NewScanner(input)
		s.scanner.Buffer(make([]byte, 0, 64*1024), maxTextLine)
		// The terminator stays in the token, so fill can tell an unterminated final
		// line from a terminated one -- which is what sed's last newline turns on.
		s.scanner.Split(scanLineWithEnding)
	}
}

// AtLast reports whether the line just returned was the final one, which is what
// the `$` address asks.
func (s *sedStream) AtLast() bool { return !s.hasHead }

// Close releases whatever input is still open, which matters when `q` stopped
// the run early.
func (s *sedStream) Close() error {
	if s.current == nil {
		return nil
	}
	err := s.current.Close()
	s.current, s.scanner = nil, nil
	return err
}
