package main

import (
	"errors"
	"io"
	"testing"
)

func TestReadBoundedInput_returnsErrNoProgress_afterRepeatedEmptyReads(t *testing.T) {
	// Given
	reader := &repeatedEmptyReader{remaining: 101}

	// When
	_, err := readBoundedInput(reader)

	// Then
	if !errors.Is(err, io.ErrNoProgress) {
		t.Fatalf("error = %v, want %v", err, io.ErrNoProgress)
	}
}

func TestReadBoundedInput_allowsOccasionalEmptyReadsBeforeData(t *testing.T) {
	// Given
	reader := &occasionalEmptyReader{}

	// When
	data, err := readBoundedInput(reader)

	// Then
	if err != nil {
		t.Fatalf("readBoundedInput() error = %v", err)
	}
	if got := string(data); got != "ok" {
		t.Fatalf("data = %q, want %q", got, "ok")
	}
}

type repeatedEmptyReader struct {
	remaining int
}

func (r *repeatedEmptyReader) Read(_ []byte) (int, error) {
	if r.remaining == 0 {
		return 0, errors.New("reader remained stalled")
	}
	r.remaining--
	return 0, nil
}

type occasionalEmptyReader struct {
	reads int
}

func (r *occasionalEmptyReader) Read(buffer []byte) (int, error) {
	r.reads++
	switch r.reads {
	case 1, 3:
		return 0, nil
	case 2:
		return copy(buffer, "ok"), nil
	default:
		return 0, io.EOF
	}
}
