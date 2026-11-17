package applets

import (
	"bytes"
	"errors"
	"io"
	"unicode"
	"unicode/utf8"

	"github.com/xiongnemo/nemosh/internal/textgrid"
)

// wcBlock is how much wc reads at once. It read through a bufio layer a rune at a time, 4096
// bytes to a read, and took 554 ms over a 27 MB file where busybox-w32 took 304.
const wcBlock = 64 << 10

// countBytes counts one stream, in bytes and in characters at once.
//
// Those are different questions on a UTF-16 file, which is why `-m` was recorded as
// outstanding: `-c` must stay a count of the bytes on disk, and `-m` has to be a count of
// the characters they encode. Counting the raw bytes on the way *into* the decoder
// answers both from a single pass -- `-c` from the tally, and `-m`, `-l`, `-w` and `-L`
// from the decoded text, which is the only view in which they mean anything.
//
// The byte-order mark is not a character. It is consumed as a mark on the way in, so `-c`
// counts its bytes and `-m` does not count it as one -- the same rule `grep` follows when
// it strips a mark rather than trying to match `^` after it. GNU counts it; this is a
// deliberate divergence and the reason is that a mark is the file saying what it is,
// rather than part of what it says.
//
// The text is counted a block at a time: an ASCII byte by what it is, and anything else as
// the character it begins, which a block that ends inside one carries into the next.
func countBytes(input io.Reader) (wcCounts, error) {
	counting := &countingReader{inner: input}
	decoded := decodeTextInput(counting)
	buffer := make([]byte, utf8.UTFMax+wcBlock)
	var counter wcCounter
	carried := 0
	for {
		read, err := decoded.Read(buffer[carried : carried+wcBlock])
		block := buffer[:carried+read]
		used := counter.count(block, err != nil)
		carried = copy(buffer, block[used:])
		if err != nil {
			if !errors.Is(err, io.EOF) {
				return wcCounts{}, err
			}
			// A final line without a newline is still a line for -L, which is the width of
			// the longest *line* rather than of the longest newline-terminated one. The tally
			// is the whole file, any bytes the decoder took as a mark included.
			counter.counts.longest = max(counter.counts.longest, counter.width)
			counter.counts.bytes = counting.total
			return counter.counts, nil
		}
	}
}

// countLinesAndBytes is countBytes when only -l and -c were asked for: the newlines in the
// decoded text, and the bytes on the way in.
func countLinesAndBytes(input io.Reader) (wcCounts, error) {
	counting := &countingReader{inner: input}
	decoded := decodeTextInput(counting)
	buffer := make([]byte, wcBlock)
	var counts wcCounts
	for {
		read, err := decoded.Read(buffer)
		counts.lines += int64(bytes.Count(buffer[:read], []byte{'\n'}))
		if err != nil {
			if !errors.Is(err, io.EOF) {
				return wcCounts{}, err
			}
			counts.bytes = counting.total
			return counts, nil
		}
	}
}

// wcCounter is a count under way: what has been counted, whether the last character was in a
// word, and how wide the line is so far.
type wcCounter struct {
	counts wcCounts
	inWord bool
	width  int64
}

// count counts block, and says how much of it was counted: all of it, but for a character
// it ends inside of, unless it is the last.
func (c *wcCounter) count(block []byte, last bool) int {
	counts, inWord, width := c.counts, c.inWord, c.width
	at := 0
	for at < len(block) {
		b := block[at]
		if b < utf8.RuneSelf {
			at++
			counts.chars++
			switch {
			case b > ' ' && b < 0x7f:
				width++
				if !inWord {
					counts.words++
					inWord = true
				}
			case b == ' ':
				width++
				inWord = false
			case b == '\n':
				counts.lines++
				counts.longest = max(counts.longest, width)
				width, inWord = 0, false
			case b == '\t':
				// A tab reaches the next multiple of eight, as busybox and GNU count it.
				width = (width | 7) + 1
				inWord = false
			case b == '\r' || b == '\f':
				// A carriage return and a form feed end a line for -L, and start the next at
				// the left, as busybox's wc has them; only a newline is a line for -l.
				counts.longest = max(counts.longest, width)
				width, inWord = 0, false
			case b == '\v':
				// A vertical tab ends one too, and the next starts where it left off.
				counts.longest = max(counts.longest, width)
				inWord = false
			default:
				// A control character takes no cell, and is part of a word.
				if !inWord {
					counts.words++
					inWord = true
				}
			}
			continue
		}
		if !last && !utf8.FullRune(block[at:]) {
			break
		}
		r, size := utf8.DecodeRune(block[at:])
		at += size
		counts.chars++
		width += int64(textgrid.RuneCells(r))
		if unicode.IsSpace(r) {
			inWord = false
		} else if !inWord {
			counts.words++
			inWord = true
		}
	}
	c.counts, c.inWord, c.width = counts, inWord, width
	return at
}
