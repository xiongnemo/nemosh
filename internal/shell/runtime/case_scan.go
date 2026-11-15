package runtime

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// The case questions, asked of one text at positions that only move forward.
//
// casePatternPosition and insideCase are asked about a prefix -- "a scan that has read this
// far, is it where a pattern goes, is a case open" -- and each read the prefix from its first
// byte. The scans ask at every bracket, so a group, a subshell or a function body holding a
// case read its text once per pattern: a case of 1000 arms inside a function took 615 ms to
// parse where the same case at the top took 8, and git-completion.bash, a file of functions
// with cases in them, took 44 ms.
//
// The answer is still computed on demand, which is the property case_pattern_position.go
// keeps on purpose: the callers are scans with a dozen `continue`s apiece, and state they had
// to update on every one of those paths would be missed on one of them. A caller asks
// patternAt or openAt with the position its own scan has reached, and these keep what they
// have read so far, so the next question starts where the last one stopped.

// casePosition is casePatternPosition's state after some text: the cases open, where the
// scan stands in one, the quote it is in, and where the word it is reading began.
type casePosition struct {
	depth   int
	pattern bool
	subject bool
	ownIn   bool
	quote   byte
	escaped bool
	word    int
}

// caseScan answers casePatternPosition(text[:end]) for one text, end only growing.
type caseScan struct {
	text  string
	at    int
	state casePosition
}

// extend is the scan with text, which goes on from what it had: the logical line a scanner is
// still building, a longer text that begins with the last one. A text that starts over is a new
// scan, which the caller makes -- flushLogicalLine does -- since a new line can grow past where
// the last one stopped before it is asked anything. A shorter text starts again regardless.
func (c *caseScan) extend(text string) {
	if len(text) < c.at {
		*c = caseScan{}
	}
	c.text = text
}

// patternAt is casePatternPosition(c.text[:end]).
//
// What a scan of the prefix decides at a byte depends on the prefix's end only where it looks
// ahead: for the close of a `$'...'`, a `$(`, a `${` or a `((`, and for the `;` after a `;`.
// A step whose look ahead was answered inside the prefix is answered the same for every longer
// one, so it is kept; from the first step that reached the end, or found nothing before it,
// the rest is read again next time, from there.
func (c *caseScan) patternAt(end int) bool {
	if end < c.at {
		c.at, c.state = 0, casePosition{}
	}
	prefix := c.text[:end]
	state, index := c.state, c.at
	keeping := true
	for index < end {
		next, ok, dependent := state.step(prefix, index)
		if dependent {
			keeping = false
		}
		if !ok {
			return false
		}
		index = next
		if keeping {
			c.state, c.at = state, index
		}
	}
	state.endWord(prefix, end)
	return state.depth > 0 && state.pattern
}

// step reads the byte at index, and the bytes a construct starting there runs to, as
// casePatternPosition's loop does. It returns where the next step begins; ok false where that
// loop gives up and answers false; and dependent where the answer turned on the prefix ending
// where it does.
func (s *casePosition) step(prefix string, index int) (int, bool, bool) {
	char := prefix[index]
	atEnd := index+1 >= len(prefix)
	if !s.escaped && s.quote == 0 {
		if end := ansiQuoteClose(prefix, index); end >= 0 {
			return end + 1, true, false
		}
	}
	switch {
	case s.escaped:
		s.escaped = false
	case char == '\\' && s.quote != '\'':
		s.escaped = true
	case char == '\'' && s.quote != '"', char == '"' && s.quote != '\'':
		if s.quote == char {
			s.quote = 0
		} else if s.quote == 0 {
			s.quote = char
		}
	case s.quote != 0:
	case char == '$' && !atEnd && (prefix[index+1] == '(' || prefix[index+1] == '{'):
		end, ok := expansionEnd(prefix, index)
		if !ok {
			return 0, false, true
		}
		return end + 1, true, false
	case char == '$':
		// `$'` with no close yet, or a `$` that ends the prefix, may be the start of a quote
		// or an expansion once more of the text is read.
		return index + 1, true, atEnd || prefix[index+1] == '\''
	case char == '(' && !atEnd && prefix[index+1] == '(':
		end := arithmeticCommandEnd(prefix, index)
		if end == 0 {
			return 0, false, true
		}
		s.endWord(prefix, index)
		s.word = end + 1
		return end + 1, true, false
	case char == ';':
		s.endWord(prefix, index)
		next := index + 1
		dependent := atEnd
		if !atEnd && prefix[next] == ';' {
			next++
			dependent = next >= len(prefix)
			if next < len(prefix) && prefix[next] == '&' {
				next++
			}
			s.pattern = s.depth > 0
		}
		s.word = next
		return next, true, dependent
	case char == ')':
		s.endWord(prefix, index)
		s.word = index + 1
		s.pattern = false
	case char == '(', char == '|', char == '&', char == ' ', char == '\t', char == '\n':
		s.endWord(prefix, index)
		s.word = index + 1
		// A `(` that ends the prefix may be the first of a `((`.
		return index + 1, true, char == '(' && atEnd
	}
	return index + 1, true, false
}

// endWord ends the word that runs from s.word to end, as casePatternPosition's does: what it
// was decides whether a pattern begins or a case ends.
func (s *casePosition) endWord(text string, end int) {
	word := ""
	if s.word < end {
		word = text[s.word:end]
	}
	afterSubject := s.ownIn
	if word != "" {
		s.ownIn = false
	}
	switch {
	case word == "":
	case s.subject:
		s.subject = false
		s.ownIn = true
	// Where a pattern goes, `case` is a pattern: `case case in case)`.
	case word == "case" && !s.pattern:
		s.depth++
		s.pattern = false
		s.subject = true
	case word == "in":
		if afterSubject {
			s.pattern = true
		}
	case word == "esac":
		if s.depth > 0 {
			s.depth--
		}
		s.pattern = false
	}
}

// caseWords answers insideCase(text[:end]) for one text, end only growing. insideCase reads
// fields, so a field followed by a blank is settled and is kept; the one the prefix ends in is
// read again, since more text may lengthen it.
type caseWords struct {
	text    string
	at      int
	depth   int
	subject bool
}

// extend is caseScan.extend, for caseWords, on the same terms.
func (c *caseWords) extend(text string) {
	if len(text) < c.at {
		*c = caseWords{}
	}
	c.text = text
}

// openAt is insideCase(c.text[:end]).
func (c *caseWords) openAt(end int) bool {
	if end < c.at {
		c.at, c.depth, c.subject = 0, 0, false
	}
	prefix := c.text[:end]
	index := c.at
	for index < end {
		start, stop := nextField(prefix, index)
		if start == stop {
			c.at = stop
			break
		}
		if stop == end {
			// The field the prefix ends in: counted, but not kept.
			depth, subject := c.depth, c.subject
			countCaseWord(prefix[start:stop], &depth, &subject)
			return depth > 0
		}
		countCaseWord(prefix[start:stop], &c.depth, &c.subject)
		index = stop
		c.at = stop
	}
	return c.depth > 0
}

// countCaseWord counts one field as insideCase does: the word after `case` is its subject and
// no keyword, whatever it says.
func countCaseWord(field string, depth *int, subject *bool) {
	word := strings.Trim(field, ";&|(){}")
	switch {
	case *subject:
		*subject = false
	case word == "case":
		*depth++
		*subject = true
	case word == "esac":
		if *depth > 0 {
			*depth--
		}
	}
}

// nextField is where the field at or after index starts and stops, as strings.Fields splits;
// start equals stop when there is none.
func nextField(text string, index int) (int, int) {
	start := index
	for start < len(text) {
		r, size := utf8.DecodeRuneInString(text[start:])
		if !unicode.IsSpace(r) {
			break
		}
		start += size
	}
	stop := start
	for stop < len(text) {
		r, size := utf8.DecodeRuneInString(text[stop:])
		if unicode.IsSpace(r) {
			break
		}
		stop += size
	}
	return start, stop
}
