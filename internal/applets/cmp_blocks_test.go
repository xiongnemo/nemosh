package applets

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"strings"
	"testing"
)

// commonPrefix is how many bytes two blocks share from their start.
func TestCmp_commonPrefix(t *testing.T) {
	a := bytes.Repeat([]byte("abcdefgh\n"), 2000)
	for _, at := range []int{0, 1, 4095, 4096, 4097, 8191, 17999} {
		b := bytes.Clone(a)
		b[at] ^= 1
		if got := commonPrefix(a, b); got != at {
			t.Errorf("a difference at %d gave %d", at, got)
		}
	}
	if got := commonPrefix(a, a); got != len(a) {
		t.Errorf("the same block gave %d of %d", got, len(a))
	}
}

// cmp compares what both inputs hold a block at a time, and answers exactly as busybox's loop of
// a byte from each did, kept here as it was: the position and line of the first difference, each
// one under -l, the EOF of the shorter, -n's limit, -s's silence. It compared 1 GiB in 6.4 s, a
// byte at a time through two bufio readers.
func TestCmp_comparesInBlocksAsItDidByteByByte(t *testing.T) {
	random := rand.New(rand.NewPCG(3, 4))
	for trial := range 300 {
		size := []int{0, 1, 15, 16, 17, 100, 4096, 70000}[trial%8] + random.IntN(3)
		first := make([]byte, size)
		for index := range first {
			first[index] = "ab\n"[random.IntN(3)]
		}
		second := bytes.Clone(first)
		for range random.IntN(4) {
			if len(second) > 0 {
				second[random.IntN(len(second))] = 'x'
			}
		}
		switch random.IntN(4) {
		case 0:
			second = second[:random.IntN(len(second)+1)]
		case 1:
			second = append(second, "tail\n"...)
		}
		for _, run := range []cmpRun{
			{names: []string{"a", "b"}, limit: -1},
			{names: []string{"a", "b"}, limit: -1, list: true},
			{names: []string{"a", "b"}, limit: -1, quiet: true},
			{names: []string{"a", "b"}, limit: random.IntN(size + 2)},
			{names: []string{"a", "b"}, limit: random.IntN(size + 2), list: true},
		} {
			bufferSize := []int{16, 17, 4096, 64 << 10}[random.IntN(4)]
			got := cmpOutcome(t, run, first, second, bufferSize, cmpRun.compare)
			want := cmpOutcome(t, run, first, second, bufferSize, compareByteByByte)
			if got != want {
				t.Fatalf("%d and %d bytes, %+v, buffers of %d:\ngot  %s\nwant %s", len(first), len(second), run, bufferSize, got, want)
			}
		}
	}
}

func cmpOutcome(t *testing.T, run cmpRun, first, second []byte, bufferSize int, compare func(cmpRun, [2]*bufio.Reader, *bufio.Writer, io.Writer) (int, error)) string {
	t.Helper()
	inputs := [2]*bufio.Reader{bufio.NewReaderSize(bytes.NewReader(first), bufferSize), bufio.NewReaderSize(bytes.NewReader(second), bufferSize)}
	var stdout, stderr strings.Builder
	out := bufio.NewWriter(&stdout)
	status, err := compare(run, inputs, out, &stderr)
	out.Flush()
	return fmt.Sprintf("status %d err %v stdout %q stderr %q", status, err, stdout.String(), stderr.String())
}

// compareByteByByte is cmp's loop as it was before it compared blocks.
func compareByteByByte(c cmpRun, inputs [2]*bufio.Reader, out *bufio.Writer, stderr io.Writer) (int, error) {
	status, line := 0, 1
	for position, limit := uint64(1), c.limit; limit != 0; position, limit = position+1, limit-1 {
		first, firstErr := inputs[0].ReadByte()
		second, secondErr := inputs[1].ReadByte()
		if firstErr != nil && !errors.Is(firstErr, io.EOF) {
			return 2, operandFailure(c.names[0], firstErr)
		}
		if secondErr != nil && !errors.Is(secondErr, io.EOF) {
			return 2, operandFailure(c.names[1], secondErr)
		}
		if firstErr == nil && secondErr == nil && first == second {
			if first == '\n' {
				line++
			}
			continue
		}
		if firstErr != nil && secondErr != nil {
			return status, nil
		}
		status = 1
		if firstErr != nil || secondErr != nil {
			if !c.quiet {
				ended := c.names[0]
				if secondErr != nil {
					ended = c.names[1]
				}
				if err := out.Flush(); err != nil {
					return 2, err
				}
				fmt.Fprintf(stderr, "cmp: EOF on %s\n", ended)
			}
			return status, nil
		}
		switch {
		case c.quiet:
			return status, nil
		case c.list:
			fmt.Fprintf(out, "%d %3o %3o\n", position, first, second)
		default:
			_, err := fmt.Fprintf(out, "%s %s differ: byte %d, line %d\n", c.names[0], c.names[1], position, line)
			return status, err
		}
	}
	return status, nil
}
