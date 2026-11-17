package applets

import (
	"io"
	"os"
)

// The end of a regular file is found from its end, as busybox's tail seeks on a file it can
// (coreutils/tail.c): read back a block at a time until the lines asked for are behind, then
// forward from there, so what is read is what is printed and the block it begins in. tail read
// the whole file a line at a time, a string for each, so `tail -n1` of 27 MB took 51 ms where
// busybox took 33. busybox seeks back 64 KB a line and trusts no line to be longer; this counts
// the line ends, so a longer line is still whole.
//
// Only what is ahead of the file's offset is the input, as it was read: `{ read x; tail -n2; }
// <file` tails what the read left. The offset is left at the end, where -f goes on from.

// tailBlock is how much is read back at a time; a test makes it small.
var tailBlock = 64 << 10

// seekableTail is the regular file input reads, if it is an operand tail may read back in.
// Standard input is read through, as a stream is.
func seekableTail(input io.Reader) *os.File {
	file, ok := operandFile(input)
	if !ok || !isRegularFile(file) {
		return nil
	}
	return file
}

// tailFromEnd prints the end spec asks for of file, answering false for a file it cannot seek in,
// which is then read through as any input is.
func tailFromEnd(stdout io.Writer, file *os.File, spec countSpec) (bool, error) {
	origin, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return false, nil
	}
	size, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return false, nil
	}
	start := max(origin, size-spec.count)
	if !spec.bytes {
		if start, err = tailLinesStart(file, origin, size, spec.count); err != nil {
			return true, err
		}
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return true, err
	}
	buffer := copyBuffers.Get().(*[]byte)
	defer copyBuffers.Put(buffer)
	_, err = io.CopyBuffer(struct{ io.Writer }{stdout}, struct{ io.Reader }{file}, *buffer)
	return true, err
}

// tailLinesStart is where the last count lines between origin and size begin: after the
// count-th line end back from the end, the file's own last one not counted, since it ends the
// last line rather than beginning another.
func tailLinesStart(file *os.File, origin, size, count int64) (int64, error) {
	if count <= 0 {
		return size, nil
	}
	buffer := make([]byte, min(int64(tailBlock), max(size-origin, 1)))
	ends := int64(0)
	for end := size; end > origin; {
		chunk := min(int64(len(buffer)), end-origin)
		at := end - chunk
		if _, err := file.ReadAt(buffer[:chunk], at); err != nil && err != io.EOF {
			return 0, err
		}
		for index := chunk - 1; index >= 0; index-- {
			if buffer[index] != '\n' || at+index == size-1 {
				continue
			}
			if ends++; ends == count {
				return at + index + 1, nil
			}
		}
		end = at
	}
	return origin, nil
}
