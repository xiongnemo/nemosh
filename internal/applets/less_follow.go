package applets

import (
	"context"
	"errors"
	"io"
	"os"
	"time"
)

// Following a FILE, F's and +F's, as tail -f follows one: at its end the reading waits and tries
// again rather than ending, for as long as following is on. A FILE whose end had been reached
// before F is opened again and read on from the line after the last one read -- a last line that
// had no newline then is taken as read, so what was added to it is not. A pipe needs none of
// this: it ends when its writer does, and until then the screen staying at the end is following.

// lessFollowPoll is how long the reading waits at a FILE's end before it looks again.
const lessFollowPoll = 200 * time.Millisecond

// lessFollowReader is a FILE read on past its end while its feed is following.
type lessFollowReader struct {
	ctx    context.Context
	feed   *lessFeed
	reader io.Reader
}

func (r lessFollowReader) Read(buffer []byte) (int, error) {
	for {
		n, err := r.reader.Read(buffer)
		if n > 0 || !errors.Is(err, io.EOF) || !r.feed.following.Load() {
			return n, err
		}
		select {
		case <-r.ctx.Done():
			return 0, r.ctx.Err()
		case <-time.After(lessFollowPoll):
		}
	}
}

// follow turns following on or off, and reads a FILE on whose end had been reached.
func (f *lessFeed) follow(on bool) {
	f.following.Store(on)
	if !on {
		return
	}
	f.mu.Lock()
	resume := f.ended && f.err == nil && f.native != ""
	skip := len(f.lines)
	if resume {
		f.ended = false
	}
	f.mu.Unlock()
	if !resume {
		return
	}
	file, err := os.Open(f.native)
	if err != nil {
		f.finish(err)
		return
	}
	go func() {
		defer file.Close()
		f.finish(readLessSource(f.ctx, lessFollowReader{ctx: f.ctx, feed: f, reader: file}, func(line string) {
			if skip > 0 {
				skip--
				return
			}
			f.add(line)
		}))
	}()
}
