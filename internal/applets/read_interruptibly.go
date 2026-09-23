package applets

import (
	"context"
	"os"
)

// ReadInterruptibly reads from file, ending the read early if ctx is cancelled while it is
// already blocked -- which checking the context before the read can never do. At a terminal
// that was `bc` ignoring Ctrl-C until the next keystroke, and consuming that keystroke to
// notice it.
//
// **One implementation, for both places that need it.** There were two, one here and one in
// internal/shell/runtime, written the same afternoon from the same idea -- and they carried
// the same data race, which is the argument for there being one. The shell's stdin reaches a
// file through descriptorReader and an applet invoked directly reaches it through
// contextReader; both now end up here.
//
// **The watcher is waited for, not merely told to stop.** It selects on the cancellation and
// on a stop signal, and a select with both ready picks either. So a watcher that had not yet
// been scheduled when the read returned could wake to find the context cancelled as well,
// choose that, and reach for the file's handle after this function had returned and its
// caller had moved on. The race detector caught exactly that on a Windows runner: the
// watcher reading the handle in Fd() while a Close() on another goroutine was destroying it.
// It surfaced once in three hundred runs locally. Waiting for the watcher means it can only
// ever act while this function still owns the read -- and at that point a cancel with nothing
// pending is harmless, because interruptBlockedRead treats ERROR_NOT_FOUND as success.
//
// An aborted console read reports EOF, which is indistinguishable from the input really
// ending, so the context decides what a zero-length read means. Bytes that did arrive are
// handed back rather than thrown away; the next read reports the cancellation.
func ReadInterruptibly(ctx context.Context, file *os.File, buffer []byte) (int, error) {
	if ctx.Done() == nil {
		// A context that can never be cancelled has nothing to watch for, and a goroutine
		// per read to watch it would be a cost with no purpose.
		return file.Read(buffer)
	}
	stop := make(chan struct{})
	watching := make(chan struct{})
	go func() {
		defer close(watching)
		select {
		case <-ctx.Done():
			interruptBlockedRead(file)
		case <-stop:
		}
	}()
	read, err := file.Read(buffer)
	close(stop)
	<-watching
	if read == 0 && ctx.Err() != nil {
		return 0, ctx.Err()
	}
	return read, err
}
