package applets

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"syscall"
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
	// A device too: `dd if=/dev/zero of=img bs=1k count=64` is how an empty image is made.
	file, err := openProcessInput(view, r.input)
	if err != nil {
		return nil, nil, cannotOpen(r.input, err)
	}
	return file, func() { file.Close() }, nil
}

func (r ddRequest) openOutput(view ProcessView, stdout io.Writer) (io.Writer, func(), error) {
	if r.output == "" {
		// seek= moves the standard output too, the shell's descriptor 1: `dd seek=1 > f`.
		if err := r.seekStandardOutput(view, stdout); err != nil {
			return nil, nil, operandFailure("standard output", err)
		}
		return stdout, func() {}, nil
	}
	// Truncated where the copy begins: at the start, or with seek= at the point it names, so
	// the blocks it passes over are kept, as POSIX and busybox's ftruncate have it. notrunc
	// shortens nothing.
	flags := os.O_WRONLY | os.O_CREATE
	if !r.notrunc && r.seek == 0 {
		flags |= os.O_TRUNC
	}
	// 0666 through the umask, as busybox's xopen makes of=. A device too: of=/dev/null.
	file, err := openProcessOutput(view, r.output, flags, createMode(view, 0o666))
	if err != nil {
		return nil, nil, cannotCreate(r.output, err)
	}
	if !r.notrunc && r.seek > 0 {
		err = truncateAt(file, r.seek*r.outputSize)
	}
	if err == nil {
		err = r.seekOutput(file)
	}
	if err != nil {
		file.Close()
		return nil, nil, operandFailure(r.output, err)
	}
	return file, func() { file.Close() }, nil
}

// seekOutput moves past seek= blocks from where the descriptor stands, as busybox's
// lseek(SEEK_CUR) does: of=/dev/stdout is the shell's, and may be part-written. What cannot
// seek, a pipe, fails as it does there.
func (r ddRequest) seekOutput(output any) error {
	if r.seek <= 0 {
		return nil
	}
	seeker, ok := output.(io.Seeker)
	if !ok {
		return syscall.ESPIPE
	}
	_, err := seeker.Seek(r.seek*r.outputSize, io.SeekCurrent)
	return err
}

// seekStandardOutput is seekOutput on the shell's descriptor 1, which the applet's own
// writer seldom seeks.
func (r ddRequest) seekStandardOutput(view ProcessView, stdout io.Writer) error {
	if _, ok := stdout.(io.Seeker); !ok && r.seek > 0 {
		if shared, err := openProcessOutput(view, "/dev/stdout", os.O_WRONLY, 0); err == nil {
			defer shared.Close()
			return r.seekOutput(shared)
		}
	}
	return r.seekOutput(stdout)
}

// truncateAt cuts the output at size, as busybox's ftruncate does. An output that cannot be
// cut, a pipe or a device, is no failure unless it is a file.
func truncateAt(output io.Writer, size int64) error {
	truncater, ok := output.(interface{ Truncate(int64) error })
	if !ok {
		return nil
	}
	err := truncater.Truncate(size)
	if err == nil {
		return nil
	}
	if file, ok := output.(interface{ Stat() (os.FileInfo, error) }); ok {
		if info, statErr := file.Stat(); statErr == nil && (info.Mode().IsRegular() || info.IsDir()) {
			return err
		}
	}
	return nil
}

// copyRecords is the loop, and it answers what to report even when it fails.
func (r ddRequest) copyRecords(ctx context.Context, source io.Reader, sink io.Writer) (ddCounts, ddCounts, error) {
	var read ddCounts
	output := ddOutput{sink: sink, size: r.outputSize, gathers: r.inputSize != r.outputSize}
	if err := r.skipInput(source); err != nil {
		return read, output.written, err
	}
	block := make([]byte, r.inputSize)
	for !r.hasCount || read.full+read.partial < r.count {
		if err := ctx.Err(); err != nil {
			return read, output.written, err
		}
		length, err := source.Read(block)
		if length > 0 {
			if int64(length) == r.inputSize {
				read.full++
			} else {
				read.partial++
			}
			if writeErr := output.write(r.convert(block[:length])); writeErr != nil {
				return read, output.written, writeErr
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return read, output.written, err
		}
	}
	err := output.finish()
	return read, output.written, err
}

// ddOutput is the writing half. With one block size a record goes out as it came in. With ibs
// and obs two sizes the records are gathered into blocks of obs, each written when it is full
// and the rest at the end, as busybox's dd does with its second buffer: `dd ibs=4 obs=8
// count=2` is "1+0 records out". Each record went out as it came, "0+2".
type ddOutput struct {
	sink    io.Writer
	size    int64
	gathers bool
	pending []byte
	written ddCounts
}

func (o *ddOutput) write(record []byte) error {
	if !o.gathers {
		return o.put(record)
	}
	for len(record) > 0 {
		taken := min(len(record), int(o.size)-len(o.pending))
		o.pending, record = append(o.pending, record[:taken]...), record[taken:]
		if int64(len(o.pending)) == o.size {
			if err := o.put(o.pending); err != nil {
				return err
			}
			o.pending = o.pending[:0]
		}
	}
	return nil
}

// finish writes what was gathered short of a block, as a partial record.
func (o *ddOutput) finish() error {
	if len(o.pending) == 0 {
		return nil
	}
	return o.put(o.pending)
}

// put writes one record and counts it, whole when it is a block of obs.
func (o *ddOutput) put(record []byte) error {
	if _, err := o.sink.Write(record); err != nil {
		return err
	}
	if int64(len(record)) == o.size {
		o.written.full++
	} else {
		o.written.partial++
	}
	return nil
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
