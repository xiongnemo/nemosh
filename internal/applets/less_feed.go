package applets

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
)

// What less has read, read as it arrives.
//
// It read the whole of its input before it drew anything, so `tail -f log | less` never drew
// and `make | less` drew when make was done. busybox's reads only as far as the screen needs
// (miscutils/less.c, read_lines), and so it draws the first page of a pipe while the rest is
// still coming. Here a goroutine reads, a line at a time, and wakes the pager, which takes
// what has arrived each time it is woken and draws it.
//
// The pager shares the reader's slice rather than copying it: a line once read is never
// written again, and the reader only appends past the end the pager was given, so the two
// touch different elements. The slice header is the one thing both write, under the lock.

// lessSource is one input: a file or standard input, and how to let it go.
type lessSource struct {
	reader io.Reader
	close  func()
}

// lessFeed is the lines read so far, and whether the reading has ended.
type lessFeed struct {
	mu    sync.Mutex
	lines []string
	ended bool
	err   error
	// raw is -R: colour sequences are taken out of a line as it is read, as busybox takes them.
	raw bool
	// changed is told of every line, for the -F wait before there is a screen; screen, once
	// set, is woken instead, and waking says a wake is already on its way.
	changed chan struct{}
	screen  tcell.Screen
	waking  atomic.Bool
	// stop ends the reading, and done is closed once the reader has stopped.
	stop context.CancelFunc
	done chan struct{}
}

// lessWake is the event the reader wakes the pager with.
type lessWake struct{ when time.Time }

func (w *lessWake) When() time.Time { return w.when }

// startLessFeed reads sources in order until they end or ctx does.
func startLessFeed(ctx context.Context, sources []lessSource, raw bool) *lessFeed {
	readCtx, cancel := context.WithCancel(ctx)
	feed := &lessFeed{raw: raw, changed: make(chan struct{}, 1), stop: cancel, done: make(chan struct{})}
	go feed.read(readCtx, sources)
	return feed
}

func (f *lessFeed) read(ctx context.Context, sources []lessSource) {
	defer close(f.done)
	var failure error
	for _, source := range sources {
		if failure == nil {
			failure = readLessSource(ctx, source.reader, f.add)
		}
		if source.close != nil {
			source.close()
		}
	}
	// Stopped is not failed: a pager that leaves before its input ends stops its reader.
	if errors.Is(failure, context.Canceled) {
		failure = nil
	}
	f.mu.Lock()
	f.ended, f.err = true, failure
	f.mu.Unlock()
	f.wake(true)
}

// readLessSource reads lines from one input, through a read its context can end.
func readLessSource(ctx context.Context, reader io.Reader, add func(string)) error {
	scanner := bufio.NewScanner(decodeTextInput(contextReader{ctx: ctx, reader: reader}))
	scanner.Buffer(make([]byte, 0, 64*1024), maxTextLine)
	for scanner.Scan() {
		add(scanner.Text())
	}
	return scanner.Err()
}

func (f *lessFeed) add(line string) {
	if f.raw {
		line = lessStripColour(line)
	}
	f.mu.Lock()
	f.lines = append(f.lines, line)
	f.mu.Unlock()
	f.wake(false)
}

// wake tells whoever is waiting that more has arrived: the -F wait, and the pager once it has
// a screen. One wake at a time is on its way, so a fast input is not a queue full of them; a
// wake the queue had no room for is tried again by the next line, and the last one, which no
// line follows, until it is taken or the pager has gone.
func (f *lessFeed) wake(last bool) {
	select {
	case f.changed <- struct{}{}:
	default:
	}
	f.mu.Lock()
	screen := f.screen
	f.mu.Unlock()
	if screen == nil || !f.waking.CompareAndSwap(false, true) {
		return
	}
	for screen.PostEvent(&lessWake{when: time.Now()}) != nil {
		f.waking.Store(false)
		if !last {
			return
		}
		time.Sleep(10 * time.Millisecond)
		if !f.waking.CompareAndSwap(false, true) {
			return
		}
	}
}

// attach starts waking screen.
func (f *lessFeed) attach(screen tcell.Screen) {
	f.mu.Lock()
	f.screen = screen
	f.mu.Unlock()
	f.wake(false)
}

// take is everything read so far, whether the reading has ended, and why it ended if it
// failed. The lines are the reader's own, cut so an append cannot reach them; see the top.
func (f *lessFeed) take() ([]string, bool, error) {
	f.waking.Store(false)
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lines[:len(f.lines):len(f.lines)], f.ended, f.err
}

// waitFor waits until more than count lines have arrived or the reading has ended, for -F,
// which cannot say whether the input fits on a screen until one or the other.
func (f *lessFeed) waitFor(ctx context.Context, count int) error {
	for {
		lines, ended, _ := f.take()
		if ended || len(lines) > count {
			return nil
		}
		select {
		case <-f.changed:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// lessStripColour is a line with its colour sequences taken out, as busybox's -R takes them
// (miscutils/less.c:550-566): ESC starts one, and digits, `[`, `;` and `m` go with it until
// the `m` that ends it. Any other byte ends it too, and is kept.
func lessStripColour(line string) string {
	if strings.IndexByte(line, 0x1b) < 0 {
		return line
	}
	out := make([]byte, 0, len(line))
	escape := false
	for index := 0; index < len(line); index++ {
		char := line[index]
		switch {
		case char == 0x1b:
			escape = true
		case escape && (char >= '0' && char <= '9' || char == '[' || char == ';' || char == 'm'):
			escape = char != 'm'
		default:
			escape = false
			out = append(out, char)
		}
	}
	return string(out)
}
