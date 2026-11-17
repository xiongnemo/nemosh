package applets

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"unicode"

	"github.com/xiongnemo/nemosh/internal/textgrid"
)

// wc reads its input a block at a time and counts it there. It read through two bufio layers
// and asked for one rune at a time, 4096 bytes to a read, and took 554 ms over a 27 MB file
// where busybox-w32 took 304; a megabyte is now sixteen reads of 64 KiB, with the 4 KiB the
// byte-order mark is looked for in before them and the read that finds the end after.
func TestWc_readsItsInputInBlocks(t *testing.T) {
	input := &readCounter{inner: bytes.NewReader(bytes.Repeat([]byte("word word\n"), 1<<20/10))}

	// When
	counts, err := countBytes(input)

	// Then
	if err != nil || counts.lines != 1<<20/10 {
		t.Fatalf("counts %+v, %v", counts, err)
	}
	if input.reads > 18 {
		t.Fatalf("a megabyte took %d reads, want at most 18", input.reads)
	}
}

// What wc counts is what it counted a rune at a time, whatever the input and however it
// arrives: the rune-at-a-time loop is kept here as the reference, and every input is fed in
// blocks of one to seven bytes and whole, so a character split across two reads is counted
// once, wherever it is split.
func TestWc_countsAsTheRuneLoopDid(t *testing.T) {
	var everyByte []byte
	for b := range 256 {
		everyByte = append(everyByte, byte(b), 'x', ' ')
	}
	inputs := map[string]string{
		"empty":                     "",
		"one line":                  "hello world\n",
		"no final newline":          "hello\nworld",
		"line ends":                 "a\rbc\fdef\vghij\nk\tl\t\tm\n",
		"tabs":                      "\tx\t\ty\n12345678\tz",
		"multibyte":                 "alpha beta\nγδ epsilon",
		"wide and combining":        "中文测试 é​ x\n",
		"other spaces":              "a\u0085b c　d\n",
		"invalid":                   "a\xffb\xc3 \xe4\xb8\n\xf0\x9f\x98",
		"control characters":        "a\x01b \x7f\x00\n",
		"every byte":                string(everyByte),
		"UTF-8 mark":                "\xef\xbb\xbfmarked text\n",
		"UTF-16LE":                  "\xff\xfeh\x00i\x00 \x00\x2d\x4e\n\x00",
		"UTF-16BE":                  "\xfe\xff\x00h\x00i\x00 \x4e\x2d\x00\n",
		"four-byte runes":           "😀 😀😀\n😀",
		"words across many lines":   strings.Repeat("one two  three\n", 50),
		"long line of wide letters": strings.Repeat("中", 3000) + "\n",
	}
	// A mark is looked for in what the first read brought, as it was, so a pipe that hands over
	// one byte at a time is not waited on for a second: the reference reads the same blocks.
	blocks := func(text string, chunk int) io.Reader {
		if chunk == 0 {
			return strings.NewReader(text)
		}
		return &fixedChunkReader{data: []byte(text), chunk: chunk}
	}
	for name, text := range inputs {
		for chunk := range 8 {
			want, err := referenceWcCount(blocks(text, chunk))
			if err != nil {
				t.Fatal(err)
			}
			got, err := countBytes(blocks(text, chunk))
			if err != nil || got != want {
				t.Errorf("%s in blocks of %d: %+v, %v; want %+v", name, chunk, got, err, want)
			}
			lines, err := countLinesAndBytes(blocks(text, chunk))
			if err != nil || lines.lines != want.lines || lines.bytes != want.bytes {
				t.Errorf("%s, lines and bytes alone, in blocks of %d: %+v, %v; want %d and %d", name, chunk, lines, err, want.lines, want.bytes)
			}
		}
	}
}

// A read that fails fails the count, as it did.
func TestWc_aFailedReadFailsTheCount(t *testing.T) {
	broken := errors.New("broken")
	for _, count := range []func(io.Reader) (wcCounts, error){countBytes, countLinesAndBytes} {
		if _, err := count(io.MultiReader(strings.NewReader("some text\n"), errorReader{broken})); !errors.Is(err, broken) {
			t.Errorf("a failed read gave %v", err)
		}
	}
}

type readCounter struct {
	inner io.Reader
	reads int
}

func (r *readCounter) Read(buffer []byte) (int, error) {
	r.reads++
	return r.inner.Read(buffer)
}

type errorReader struct{ err error }

func (r errorReader) Read([]byte) (int, error) { return 0, r.err }

// referenceWcCount is the loop countBytes was, a rune at a time through bufio.
func referenceWcCount(input io.Reader) (wcCounts, error) {
	counting := &countingReader{inner: input}
	reader := bufio.NewReader(decodeTextInput(counting))
	counts := wcCounts{}
	inWord := false
	lineWidth := int64(0)
	for {
		r, size, err := reader.ReadRune()
		if size > 0 {
			counts.chars++
		}
		switch {
		case r == '\n' || r == '\r' || r == '\f' || r == '\v':
			counts.longest = max(counts.longest, lineWidth)
			if r == '\n' {
				counts.lines++
			}
			if r != '\v' {
				lineWidth = 0
			}
		case r == '\t':
			lineWidth = (lineWidth | 7) + 1
		case size > 0:
			lineWidth += int64(textgrid.RuneCells(r))
		}
		if size > 0 && unicode.IsSpace(r) {
			inWord = false
		} else if size > 0 && !inWord {
			counts.words++
			inWord = true
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				counts.longest = max(counts.longest, lineWidth)
				counts.bytes = counting.total
				return counts, nil
			}
			return wcCounts{}, err
		}
	}
}
