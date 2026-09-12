package applets

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
)

// dd's copy loop.
//
// The thing that makes dd dd is that it counts **records**, not bytes, and reports whole and
// partial ones separately: `3+1 records in` means three full blocks and one short. That is
// not decoration -- a short read from a pipe is how a script learns its `bs` was optimistic.
//
// A **short read is still a record**. dd does not gather reads into full blocks unless
// `conv=sync` asks it to, which is why `dd bs=1M` on a pipe so often copies less than
// expected: each read is one record whatever its length. Reproduced rather than smoothed
// over, because a script counting records has to see what the reference would have shown it.

type ddCounts struct{ full, partial int64 }

func (c ddCounts) String() string { return fmt.Sprintf("%d+%d", c.full, c.partial) }

func (r ddRequest) run(ctx context.Context, view ProcessView, stdin io.Reader, stdout, stderr io.Writer) error {
	source, closeSource, err := r.openInput(view, stdin)
	if err != nil {
		return err
	}
	defer closeSource()
	sink, closeSink, err := r.openOutput(view, stdout)
	if err != nil {
		return err
	}
	defer closeSink()

	read, written, err := r.copyRecords(ctx, source, sink)
	if !r.silent {
		// On stderr, so the counts do not join the data in a pipeline.
		fmt.Fprintf(stderr, "%s records in\n%s records out\n", read, written)
	}
	return err
}

func (r ddRequest) openInput(view ProcessView, stdin io.Reader) (io.Reader, func(), error) {
	if r.input == "" {
		return stdin, func() {}, nil
	}
	native, err := resolveHostPath(view, r.input)
	if err != nil {
		return nil, nil, err
	}
	file, err := os.Open(native)
	if err != nil {
		return nil, nil, cannotOpen(r.input, err)
	}
	return file, func() { file.Close() }, nil
}

func (r ddRequest) openOutput(view ProcessView, stdout io.Writer) (io.Writer, func(), error) {
	if r.output == "" {
		return stdout, func() {}, nil
	}
	native, err := resolveHostPath(view, r.output)
	if err != nil {
		return nil, nil, err
	}
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if r.notrunc {
		// The point of notrunc: write into the file without shortening what follows.
		flags = os.O_WRONLY | os.O_CREATE
	}
	file, err := os.OpenFile(native, flags, 0o644)
	if err != nil {
		return nil, nil, cannotCreate(r.output, err)
	}
	if r.seek > 0 {
		if _, err := file.Seek(r.seek*r.outputSize, io.SeekStart); err != nil {
			file.Close()
			return nil, nil, operandFailure(r.output, err)
		}
	}
	return file, func() { file.Close() }, nil
}

// copyRecords is the loop, and it answers what to report even when it fails.
func (r ddRequest) copyRecords(ctx context.Context, source io.Reader, sink io.Writer) (ddCounts, ddCounts, error) {
	var read, written ddCounts
	if err := r.skipInput(source); err != nil {
		return read, written, err
	}
	block := make([]byte, r.inputSize)
	for !r.hasCount || read.full+read.partial < r.count {
		if err := ctx.Err(); err != nil {
			return read, written, err
		}
		length, err := source.Read(block)
		if length > 0 {
			if int64(length) == r.inputSize {
				read.full++
			} else {
				read.partial++
			}
			piece := r.convert(block[:length])
			if _, writeErr := sink.Write(piece); writeErr != nil {
				return read, written, writeErr
			}
			if int64(len(piece)) == r.outputSize {
				written.full++
			} else {
				written.partial++
			}
		}
		if err == io.EOF {
			return read, written, nil
		}
		if err != nil {
			return read, written, err
		}
	}
	return read, written, nil
}

// skipInput moves past the first `skip` input blocks.
//
// Seeked where the source can seek and read where it cannot, because `skip` has to work on a
// pipe as well as on a file -- and reading is the only way to skip a pipe.
func (r ddRequest) skipInput(source io.Reader) error {
	if r.skip <= 0 {
		return nil
	}
	offset := r.skip * r.inputSize
	if seeker, ok := source.(io.Seeker); ok {
		_, err := seeker.Seek(offset, io.SeekStart)
		return err
	}
	_, err := io.CopyN(io.Discard, source, offset)
	if err == io.EOF {
		// Skipping past the end leaves nothing to copy, which is not a failure.
		return nil
	}
	return err
}

// convert applies the conv= list to one record.
func (r ddRequest) convert(record []byte) []byte {
	out := record
	if r.swab {
		// Every pair of bytes exchanged, which is what reading a file written by a
		// machine of the other endianness needs. An odd trailing byte is left alone.
		out = append([]byte(nil), record...)
		for index := 0; index+1 < len(out); index += 2 {
			out[index], out[index+1] = out[index+1], out[index]
		}
	}
	switch {
	case r.upper:
		out = bytes.ToUpper(out)
	case r.lower:
		out = bytes.ToLower(out)
	}
	if r.syncPad && int64(len(out)) < r.inputSize {
		// conv=sync pads a short record to the full block with NULs, which is what makes
		// `dd` on a tape or a character device produce fixed-size records.
		padded := make([]byte, r.inputSize)
		copy(padded, out)
		out = padded
	}
	return out
}
