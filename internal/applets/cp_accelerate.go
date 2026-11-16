package applets

import (
	"context"
	"io"
	"io/fs"
	"os"
	"sync"
)

// copyAccelerated is how a large regular file is copied whole; a test makes it count.
var copyAccelerated = copyFileAccelerated

// copyAccelerated copies a large regular file by CopyFile2: it answers whether it settled the
// copy, made or failed, and leaves anything CopyFile2 refuses to copyRegular's own reading and
// writing, which says in its words what fails there.
func (r *cpRun) copyAccelerated(source, dest pathOperand, info os.FileInfo) (bool, bool) {
	if source.device || dest.device || !info.Mode().IsRegular() || info.Size() < cpAccelerateAt {
		return false, false
	}
	err := copyAccelerated(r.context(), source.host, dest.host)
	if isFileExists(err) {
		// What is in the way is removed and the copy made afresh, as copyRegular's O_EXCL has it.
		if answer := r.askAndUnlink(dest, fs.ErrExist); answer != unlinked {
			return true, answer == unlinkDeclined
		}
		err = copyAccelerated(r.context(), source.host, dest.host)
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
		return false, false
	}
	if err := settleCopiedFile(dest.host, info.Mode(), r.flags.preserve); err != nil {
		return true, r.fail(cannotOpen(dest.operand, err))
	}
	r.finish(source, dest, info)
	return true, true
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
