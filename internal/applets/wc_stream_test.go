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
	if want := (wcCounts{lines: 1, words: 4, bytes: len("alpha\u2003beta\nγδ epsilon")}); counts != want {
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
	if want := (wcCounts{lines: repetitions, words: repetitions, bytes: 5 * repetitions}); counts != want {
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
