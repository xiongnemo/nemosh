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
	// onOpenError is how an unreadable operand is reported. busybox warns and
	// carries on with status 1 (editors/sed.c:1061), so the stream keeps going.
	onOpenError func(error)
}

func (s *sedStream) Next() (string, int, bool, error) {
	if !s.primed {
		if err := s.fill(); err != nil {
			return "", 0, false, err
		}
		s.primed = true
	}
	if !s.hasHead {
		return "", 0, false, nil
	}
	line := s.head
	if err := s.fill(); err != nil {
		return "", 0, false, err
	}
	return line, 0, true, nil
}

// fill reads the next line into head, crossing into the following input when the
// current one runs out.
func (s *sedStream) fill() error {
	for {
		if s.scanner != nil {
			if s.scanner.Scan() {
				s.head, s.hasHead = s.scanner.Text(), true
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
