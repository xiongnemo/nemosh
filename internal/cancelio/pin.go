// Package cancelio ends the call a cancelled command is still waiting in.
//
// An applet runs in the shell's process, on a goroutine, and a goroutine waiting in a Windows
// call does not see its context end. Ctrl-C one second into `ls //192.0.2.30/share/x`, a share
// that does not answer, brought nemosh's prompt back 102.4 s later, when Windows gave up, and
// busybox-w32's 17 ms later: its applet is a process of its own, which the console's Ctrl-C
// ends. CancelSynchronousIo ends such a wait -- an open or a read of a share, of a pipe, of a
// file someone holds an oplock on -- at once, with ERROR_OPERATION_ABORTED, which an applet
// reports as busybox reports EINTR. But only on the thread that is waiting, and a goroutine
// has a thread of its own only while it is pinned to one.
package cancelio

import (
	"context"
	"os"
	"runtime"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

// interval is how long a thread stays stopped before its call is cancelled again: how long a
// call that begins to wait after the first cancel waits, against busybox's 17 ms.
const interval = 10 * time.Millisecond

// Pin holds the calling goroutine to its thread until release is called, and meanwhile ctx
// ending cancels the call the thread is waiting in: at once, and then again whenever the
// thread has not run at all for an interval. Again, because a call cancelled is often followed
// by another -- os.Stat, refused by GetFileAttributesEx, tries CreateFile -- and an applet given
// ten names goes on to the next; only when it has not run, so that a diagnostic the command
// prints on its way out is left to be written.
//
// release must be called on the goroutine that called Pin, and before the shell runs anything
// of its own, a trap above all: a `read` in an INT trap waits at the console, and would be
// cancelled each time it began. A pinned goroutine pays for each wait on a channel, 11.8 us a
// round trip where a free one pays 0.46, so one that waits on others lets go for it; see
// Unpinned. A pipe's reads cost what they did.
//
// NEMOSH_PIN=off pins nothing, and a call waits for Windows to answer, as before 2026-11-19.
func Pin(ctx context.Context) (release func()) {
	done := ctx.Done()
	if done == nil || pinningOff() {
		return func() {}
	}
	runtime.LockOSThread()
	held := &pin{done: done}
	if !push(held) {
		runtime.UnlockOSThread()
		return func() {}
	}
	stopWatching := context.AfterFunc(ctx, held.watch)
	return func() {
		pop(held)
		// A watcher that has started finds the pin released, under the mutex, and touches
		// the thread no more, which from here may run any goroutine.
		held.mu.Lock()
		held.released = true
		held.mu.Unlock()
		stopWatching()
		runtime.UnlockOSThread()
	}
}

// setting is NEMOSH_PIN, read once, as the shell starts its first command: os.Getenv is a call
// to Windows each time, 1.8 us, which was more than the rest of a pin.
var setting struct {
	once sync.Once
	off  bool
}

func pinningOff() bool {
	setting.once.Do(func() { setting.off = os.Getenv("NEMOSH_PIN") == "off" })
	return setting.off
}

type pin struct {
	done <-chan struct{}
	// outer is the pin held on the same thread before this one, and id the thread's; both are
	// the registry's, and guarded by its mutex.
	outer *pin
	id    uint32

	mu sync.Mutex
	// thread is the registry's handle of the thread, which it keeps; 0 is none to cancel.
	thread windows.Handle
	// released is the pin let go; suspended is it let go for a while, by Unpinned.
	released, suspended bool
	// last is the thread's cycles at the last look.
	last uint64
}

// watch cancels the thread's call once the context has ended, and again each interval the
// thread has not run in, until the pin is released.
func (p *pin) watch() {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for first := true; p.look(first); first = false {
		<-ticker.C
	}
}

// look cancels the thread's call on the first look, or when the thread has run no cycle since
// the last, and answers whether the pin is still held.
func (p *pin) look(first bool) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.released {
		return false
	}
	if p.suspended || p.thread == 0 {
		return true
	}
	cycles, known := cycleTime(p.thread)
	if first || !known || cycles == p.last {
		_ = Cancel(p.thread)
	}
	p.last = cycles
	return true
}

// currentThread is a handle of the calling thread's own, which the registry keeps.
func currentThread() (windows.Handle, error) {
	process := windows.CurrentProcess()
	var thread windows.Handle
	err := windows.DuplicateHandle(process, windows.CurrentThread(), process, &thread, 0, false, windows.DUPLICATE_SAME_ACCESS)
	return thread, err
}
