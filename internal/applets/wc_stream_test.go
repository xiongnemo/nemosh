package applets

import (
	"errors"
	"io"
	"testing"
)

func TestFinalSecurity_countBytes_preservesWordsAcrossChunkBoundaries(t *testing.T) {
	// Given
	input := &fixedChunkReader{data: []byte("alpha\u2003beta\nγδ epsilon"), chunk: 2}

	// When
	counts, err := countBytes(input)

	// Then
	if err != nil {
		t.Fatalf("count chunked input: %v", err)
	}
	// The multi-byte characters are why chars and bytes differ here, which is
	// exactly what -m exists to report: an em space and two Greek letters are
	// three characters and seven bytes between them.
	text := "alpha\u2003beta\nγδ epsilon"
	want := wcCounts{
		lines:   1,
		words:   4,
		bytes:   len(text),
		chars:   len([]rune(text)),
		longest: len([]rune("γδ epsilon")),
	}
	if counts != want {
		t.Fatalf("chunked counts: got %+v want %+v", counts, want)
	}
}

func TestFinalSecurity_countBytes_countsLargeBoundedInputIncrementally(t *testing.T) {
	// Given
	const repetitions = 1 << 20
	input := &boundedRequestReader{remaining: repetitions * 5, pattern: []byte("word\n"), maxRequest: 32 * 1024}

	// When
	counts, err := countBytes(input)

	// Then
	if err != nil {
		t.Fatalf("count large input: %v", err)
	}
	// chars and longest are counted in the same pass now, so they are part of the
	// expectation rather than left at zero: five ASCII bytes per repetition means
	// characters equal bytes, and every line is four characters wide.
	want := wcCounts{
		lines:   repetitions,
		words:   repetitions,
		bytes:   5 * repetitions,
		chars:   5 * repetitions,
		longest: 4,
	}
	if counts != want {
		t.Fatalf("large counts: got %+v want %+v", counts, want)
	}
}

type fixedChunkReader struct {
	data  []byte
	chunk int
}

type boundedRequestReader struct {
	remaining  int
	pattern    []byte
	position   int
	maxRequest int
}

func (r *fixedChunkReader) Read(buffer []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	count := min(len(r.data), r.chunk, len(buffer))
	copy(buffer, r.data[:count])
	r.data = r.data[count:]
	return count, nil
}

func (r *boundedRequestReader) Read(buffer []byte) (int, error) {
	if len(buffer) > r.maxRequest {
		return 0, errors.New("reader requested an unbounded growth buffer")
	}
	if r.remaining == 0 {
		return 0, io.EOF
	}
	count := min(len(buffer), r.remaining)
	for index := range count {
		buffer[index] = r.pattern[r.position]
		r.position = (r.position + 1) % len(r.pattern)
	}
	r.remaining -= count
	return count, nil
}
