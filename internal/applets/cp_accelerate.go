package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sync"

	"golang.org/x/sys/windows"
)

// copyAccelerated is how a large regular file is copied whole; a test makes it count.
var copyAccelerated = copyFileAccelerated

// removeCopyLeftover removes what a failed CopyFile2 left at the destination; a test makes it fail.
var removeCopyLeftover = os.Remove

// copyAccelerated copies a large regular file by CopyFile2: it answers whether it settled the
// copy, made or failed, and leaves anything CopyFile2 refuses to copyRegular's own reading and
// writing, which says in its words what fails there.
//
// mv's copy across volumes is CopyFile2's for a file of any size, and keeps what CopyFile2
// carries -- the alternate data streams, the attributes, the time -- since those are what a
// rename on one volume keeps, and a move should not keep less for crossing a drive.
//
// cp's copy to or from a share is never CopyFile2's. CopyFile2 sizes the file it makes before
// it copies a byte, and Windows keeps the handle across a lost link: on 2026-11-19 a copy onto
// a NAS that a 143 s outage ended left, once the link came back, a file of the full 8 MiB whose
// last 5.5 MiB were zeros -- a copy that looks finished. It also looks at Ctrl-C only between
// the pieces it has under way, which on a link at 1 MB/s was 24 s. copyRegular's file grows
// as it is written, as busybox's does, and stops at the next megabyte.
func (r *cpRun) copyAccelerated(source, dest pathOperand, info os.FileInfo) (bool, bool) {
	if source.device || dest.device || !info.Mode().IsRegular() || info.Size() < cpAccelerateAt && !r.flags.move {
		return false, false
	}
	if !r.flags.move && (pathIsRemote(source.host) || pathIsRemote(dest.host)) {
		return false, false
	}
	err := copyAccelerated(r.context(), source.host, dest.host, r.flags.move)
	if isFileExists(err) {
		// What is in the way is removed and the copy made afresh, as copyRegular's O_EXCL has it.
		if answer := r.askAndUnlink(dest, fs.ErrExist); answer != unlinked {
			return true, answer == unlinkDeclined
		}
		err = copyAccelerated(r.context(), source.host, dest.host, r.flags.move)
	}
	switch {
	case err == nil:
	case r.context().Err() != nil || err == errorRequestAborted:
		_ = os.Remove(dest.host)
		cause := r.context().Err()
		if cause == nil {
			cause = context.Canceled
		}
		return true, r.fail(cause)
	default:
		// What CopyFile2 made before it failed is the size of the whole file, so it goes before the
		// bytes are copied another way. One that cannot be removed -- a share that has gone away --
		// is the copy's failure, named for what CopyFile2 met rather than for what the next open
		// meets, which on a lost share is "No such file or directory".
		if removeErr := removeCopyLeftover(dest.host); removeErr != nil && !isNothingThere(removeErr) {
			return true, r.fail(fmt.Errorf("error writing to '%s': %s", dest.operand, causeText(err)))
		}
		return false, false
	}
	if !r.flags.move {
		if err := settleCopiedFile(dest.host, info.Mode(), r.flags.keep.times); err != nil {
			return true, r.fail(cannotOpen(dest.operand, err))
		}
	}
	r.finish(source, dest, info)
	return true, true
}

// isNothingThere is a removal that found no file. Go counts a network path that cannot be found
// as not existing too, which here is the share being gone, so only the two local answers count.
func isNothingThere(err error) bool {
	return errors.Is(err, windows.ERROR_FILE_NOT_FOUND) || errors.Is(err, windows.ERROR_PATH_NOT_FOUND)
}

func (r *cpRun) context() context.Context {
	if r.ctx == nil {
		return context.Background()
	}
	return r.ctx
}

// copyBuffers are the megabyte cp reads and writes the files CopyFile2 does not copy a piece
// at a time, where io.Copy read 32 KB.
var copyBuffers = sync.Pool{New: func() any { buffer := make([]byte, 1<<20); return &buffer }}

// copyFileBytes copies reader to writer a megabyte at a time, and stops between reads when ctx
// is cancelled. Wrapped, so neither side's ReadFrom or WriteTo goes around the buffer.
func copyFileBytes(ctx context.Context, writer io.Writer, reader io.Reader) (int64, error) {
	buffer := copyBuffers.Get().(*[]byte)
	defer copyBuffers.Put(buffer)
	return io.CopyBuffer(struct{ io.Writer }{writer}, contextReader{ctx: ctx, reader: reader}, *buffer)
}
