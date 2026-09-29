package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

// tailFollower is tail -f, busybox's `if (FOLLOW) while (1)`: every -s seconds each FILE's
// new bytes, from its start again when it has shrunk, and a header when the output turns to
// another FILE. With -F each FILE's name is looked at too, and a FILE it has come to name, or
// begun to, is read; the one it named before is read to its end first.
type tailFollower struct {
	ctx            context.Context
	follow         tailFollow
	files          []*tailFile
	headed         bool
	stdout, stderr io.Writer
	// last is the FILE the output last came from. busybox's prev_fd is the last FILE named
	// instead, so with it one -F waits for, its tail -F headed the first new line of the FILE it
	// had just printed.
	last int
}

func (f *tailFollower) run(ctx context.Context) error {
	f.ctx = ctx
	buffer := make([]byte, 8192)
	for {
		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		case <-time.After(time.Duration(f.follow.period) * time.Second):
		}
		for index, file := range f.files {
			var next *os.File
			if f.follow.retry {
				next = f.look(file)
			}
			if !file.open() {
				continue
			}
			if err := f.drain(index, file, next, buffer); err != nil {
				return err
			}
		}
	}
}

// drain writes what has been added to a FILE since the last look, and then what its
// replacement holds, when -F found one.
func (f *tailFollower) drain(index int, file *tailFile, next *os.File, buffer []byte) error {
	for {
		if file.file != nil {
			// A FILE shorter than where tail is has been truncated: its start is new.
			if info, err := file.file.Stat(); err == nil && info.Size() > 0 {
				if at, err := file.file.Seek(0, io.SeekCurrent); err == nil && info.Size() < at {
					file.file.Seek(0, io.SeekStart)
				}
			}
		}
		n, err := file.reader.Read(buffer)
		if n <= 0 {
			if err != nil && !errors.Is(err, io.EOF) {
				fmt.Fprintf(f.stderr, "tail: read error: %s\n", causeText(err))
			}
			if next == nil {
				return nil
			}
			file.adopt(f.ctx, next)
			next = nil
			continue
		}
		if f.headed && index != f.last {
			if _, err := io.WriteString(f.stdout, "\n"+headTailHeader(file.name, true)); err != nil {
				return err
			}
			f.last = index
		}
		if _, err := f.stdout.Write(buffer[:n]); err != nil {
			return err
		}
	}
}

// look is -F's look at a FILE's name. When it names another file than the one open, or one
// where none was open, that one is opened, and said to have been; when it names none, that is
// said. The new file of a replaced one is returned, to be read once the old is read out.
func (f *tailFollower) look(file *tailFile) *os.File {
	if file.native == "" || file.file != nil && sameFileAs(file.file, file.native) {
		return nil
	}
	next, err := openShared(file.native)
	switch {
	case err == nil && !file.open():
		fmt.Fprintf(f.stderr, "tail: %s has appeared; following end of new file\n", file.name)
		file.adopt(f.ctx, next)
	case err == nil:
		fmt.Fprintf(f.stderr, "tail: %s has been replaced; following end of new file\n", file.name)
		return next
	case file.open():
		fmt.Fprintf(f.stderr, "tail: %s has been renamed or deleted: %s\n", file.name, causeText(err))
	}
	return nil
}

// sameFileAs is whether native names the file that is open, busybox's st_dev and st_ino test.
func sameFileAs(file *os.File, native string) bool {
	open, err := file.Stat()
	if err != nil {
		return false
	}
	named, err := os.Stat(native)
	return err == nil && os.SameFile(open, named)
}
