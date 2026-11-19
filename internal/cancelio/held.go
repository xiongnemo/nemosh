package cancelio

import (
	"context"
	"runtime"
	"sync"

	"golang.org/x/sys/windows"
)

// held is the pins held on each thread, the innermost first, so that a goroutine can find its
// own by the thread it is on: a pinned thread runs no goroutine but the one pinned to it, so
// the pins of the thread a goroutine finds itself on are that goroutine's. A pin is entered
// once its goroutine is locked to the thread and taken out before it is unlocked.
//
// threads is a handle of each thread a pin has held, kept for good: making one and closing it
// again was 0.6 us of each pin. Holding it is also what keeps the thread's id from being given
// to another thread, should this one end.
var held struct {
	sync.Mutex
	pins    map[uint32]*pin
	threads map[uint32]windows.Handle
}

// push enters p as the innermost pin of the calling thread, which it is locked to, and answers
// false when there is no handle of the thread to cancel its calls with.
func push(p *pin) bool {
	id := windows.GetCurrentThreadId()
	held.Lock()
	defer held.Unlock()
	thread, ok := threadLocked(id)
	if !ok {
		return false
	}
	p.thread, p.id, p.outer = thread, id, held.pins[id]
	held.pins[id] = p
	return true
}

// threadLocked is the handle of the calling thread, whose id is id, made the first time; held's
// mutex is held.
func threadLocked(id uint32) (windows.Handle, bool) {
	if held.pins == nil {
		held.pins = map[uint32]*pin{}
		held.threads = map[uint32]windows.Handle{}
	}
	if thread, ok := held.threads[id]; ok {
		return thread, true
	}
	thread, err := currentThread()
	if err != nil {
		return 0, false
	}
	held.threads[id] = thread
	return thread, true
}

func pop(p *pin) {
	held.Lock()
	defer held.Unlock()
	link := held.pins[p.id]
	if link == p {
		if p.outer == nil {
			delete(held.pins, p.id)
		} else {
			held.pins[p.id] = p.outer
		}
		return
	}
	for ; link != nil; link = link.outer {
		if link.outer == p {
			link.outer = p.outer
			return
		}
	}
}

// Covers is whether the calling goroutine is pinned with ctx, so that a read it makes is ended
// by the pin when ctx ends and needs no watcher of its own. One was a goroutine and a wait on
// its channel at each read, which a pinned goroutine paid 12.7 us for: `while read` over 20,000
// lines on a pipe took 4.9 times as long pinned.
func Covers(ctx context.Context) bool {
	done := ctx.Done()
	if done == nil {
		return false
	}
	id := windows.GetCurrentThreadId()
	held.Lock()
	defer held.Unlock()
	for p := held.pins[id]; p != nil; p = p.outer {
		if p.done == done {
			return true
		}
	}
	return false
}

// Unpinned runs wait with the calling goroutine's pins suspended and its thread let go, and
// afterwards holds them to whichever thread it is on. It is for two things:
//   - a wait on other goroutines that does no I/O of its own to cancel: a pipeline's, whose
//     stages are pinned each on its own. A loop of 2000 pipelines took 24% longer with the
//     shell's goroutine pinned through each wait.
//   - a command that goes on through the interrupt that ended the context it was pinned with:
//     tee -i, whose read of a pipe the pin cancelled at the ^C.
func Unpinned(wait func()) {
	id := windows.GetCurrentThreadId()
	held.Lock()
	innermost := held.pins[id]
	delete(held.pins, id)
	held.Unlock()
	if innermost == nil {
		wait()
		return
	}
	depth := 0
	for p := innermost; p != nil; p = p.outer {
		p.mu.Lock()
		p.suspended = true
		p.mu.Unlock()
		depth++
	}
	for range depth {
		runtime.UnlockOSThread()
	}
	defer resume(innermost, depth)
	wait()
}

// resume pins the goroutine again and gives each of its pins the thread it is on now. A pin
// that cannot have it cancels nothing more, rather than the thread it had, which is anyone's.
func resume(innermost *pin, depth int) {
	for range depth {
		runtime.LockOSThread()
	}
	id := windows.GetCurrentThreadId()
	held.Lock()
	defer held.Unlock()
	thread, ok := threadLocked(id)
	if !ok {
		thread = 0
	}
	for p := innermost; p != nil; p = p.outer {
		p.mu.Lock()
		p.thread, p.id = thread, id
		p.suspended = false
		p.mu.Unlock()
	}
	held.pins[id] = innermost
}
