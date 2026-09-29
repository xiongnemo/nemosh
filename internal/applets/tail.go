package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
)

// tailFile is one FILE of tail's, busybox's fds[i] and argv[i]: the name it was given, where it
// is on the host, and what is open of it, with nothing open for one that -F waits for.
type tailFile struct {
	name, native string
	reader       io.Reader
	// file is the reader when it is a file tail can seek in and stat, and owned whether tail
	// opened it, and so closes it. stdin is never closed, and is a file only when it is a
	// regular one, leased from the shell until release.
	file    *os.File
	owned   bool
	release func()
}

func (t *tailFile) open() bool { return t.reader != nil }

func (t *tailFile) close() error {
	var err error
	if closer, ok := t.reader.(io.Closer); ok && t.owned {
		err = closer.Close()
	}
	if t.release != nil {
		t.release()
	}
	t.reader, t.file, t.owned, t.release = nil, nil, false, nil
	return err
}

// stdinTailFile is stdin as tail reads it, a file it can follow when it is a regular one.
func stdinTailFile(ctx context.Context, stdin io.Reader) *tailFile {
	input := &tailFile{name: "standard input", reader: stdin}
	if file, release, ok := leaseTopStdin(ctx, stdin); ok {
		if isRegularFile(file) {
			input.file, input.release = file, release
		} else {
			release()
		}
	}
	return input
}

// runTail is busybox's tail_main (coreutils/tail.c): every FILE opened first, each that cannot
// be named then, and the rest printed in turn, a header over each when more than one opened;
// and with -f, followed as they grow. It printed as head does, a FILE at a time, its headers
// counted from the FILEs named rather than the ones that opened: `tail nosuch a` headed a,
// and named nosuch among the output rather than before it.
func runTail(ctx context.Context, spec countSpec, headers headerMode, follow tailFollow, paths []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(paths) == 0 {
		input := stdinTailFile(ctx, stdin)
		defer input.close()
		// A pipe is not followed, for it has no end to wait at; busybox fstats stdin for that.
		if input.file == nil {
			follow.follow = false
		}
		return tailFiles(ctx, spec, headers, follow, []*tailFile{input}, false, stdout, stderr)
	}
	view := ProcessViewFromContext(ctx)
	var files []*tailFile
	failed := false
	for _, path := range paths {
		opened, err := openTailFile(ctx, view, path, stdin, follow.follow)
		if err != nil {
			fmt.Fprintf(stderr, "tail: %v\n", cannotOpen(path, err))
			failed = true
			if !follow.retry {
				continue
			}
		}
		files = append(files, opened)
	}
	defer func() {
		for _, file := range files {
			file.close()
		}
	}()
	if len(files) == 0 {
		return errors.New("no files")
	}
	return tailFiles(ctx, spec, headers, follow, files, failed, stdout, stderr)
}

// tailFiles prints the end of each FILE, and follows them with -f.
func tailFiles(ctx context.Context, spec countSpec, headers headerMode, follow tailFollow, files []*tailFile, failed bool, stdout, stderr io.Writer) error {
	headed, first, last := wantsHeader(headers, len(files)), true, -1
	for index, file := range files {
		if !file.open() {
			continue
		}
		last = index
		if headed {
			if _, err := io.WriteString(stdout, headTailHeader(file.name, first)); err != nil {
				return err
			}
			first = false
		}
		// Without -f each FILE is done with once printed, and a failure to close it is one of
		// the command's too.
		var closeErr error
		copyErr := copyTailOf(stdout, file.reader, spec)
		if !follow.follow {
			closeErr = file.close()
		}
		if err := errors.Join(copyErr, closeErr); err != nil {
			return err
		}
	}
	if follow.follow {
		return (&tailFollower{follow: follow, files: files, headed: headed, last: last, stdout: stdout, stderr: stderr}).run(ctx)
	}
	if failed {
		return ExitStatus(1)
	}
	return nil
}

// openTailFile opens a FILE to be tailed as any applet opens one, but for one on the host that
// is to be followed: that is opened where it is, so that another program may rename it as tail
// reads it, and -F may reopen it by name.
func openTailFile(ctx context.Context, view ProcessView, path string, stdin io.Reader, follows bool) (*tailFile, error) {
	if path == "-" {
		return stdinTailFile(ctx, stdin), nil
	}
	opened := &tailFile{name: path}
	if native, err := resolveHostPath(view, path); follows && err == nil {
		opened.native = native
		file, err := openShared(native)
		if err == nil {
			opened.adopt(ctx, file)
		}
		return opened, err
	}
	reader, err := OpenProcessInput(ctx, view, path)
	if err == nil {
		opened.reader, opened.owned = reader, true
	}
	return opened, err
}

// adopt makes file the FILE's, in place of any it had: read so that an interrupt ends a read,
// and sought and stat'd as it is.
func (t *tailFile) adopt(ctx context.Context, file *os.File) {
	t.close()
	t.reader, t.file, t.owned = &contextReadCloser{ctx: ctx, input: file}, file, true
}

func isRegularFile(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode().IsRegular()
}
