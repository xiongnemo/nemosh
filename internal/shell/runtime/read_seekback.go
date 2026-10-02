package runtime

import (
	"io"
	"os"
)

// seekBackReader serves read's bytes from a regular file a block at a time, and gives back
// what the line did not use by seeking the file back over it, as bash reads a file it can seek
// in. read takes one byte at a time, so that a command after it finds the file where the line
// ended; each byte was a read of its own, through the reader that can be interrupted, so
// `while read l; do :; done < f` took twice busybox's time over twenty thousand lines. A file
// on disk never makes a read wait, which is the one thing that reader is for.
type seekBackReader struct {
	file  *os.File
	block [512]byte
	held  []byte
}

// regularFile is input when it is a file on disk, the only kind it is safe to read ahead in.
func regularFile(input io.Reader) (*os.File, bool) {
	file, ok := input.(*os.File)
	if !ok {
		return nil, false
	}
	info, err := file.Stat()
	return file, err == nil && info.Mode().IsRegular()
}

func (r *seekBackReader) Read(buffer []byte) (int, error) {
	if len(r.held) == 0 {
		count, err := r.file.Read(r.block[:])
		if count == 0 {
			return 0, err
		}
		r.held = r.block[:count]
	}
	count := copy(buffer, r.held)
	r.held = r.held[count:]
	return count, nil
}

// giveBack seeks the file back over the bytes read ahead and not used.
func (r *seekBackReader) giveBack() {
	if len(r.held) > 0 {
		_, _ = r.file.Seek(-int64(len(r.held)), io.SeekCurrent)
		r.held = nil
	}
}
