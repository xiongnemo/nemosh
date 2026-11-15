package applets

import (
	"context"
	"fmt"
	"io"

	"github.com/gdamore/tcell/v2"
)

// Running the pager: the screen, the loop, and what the feed has read; see less_feed.go. Its own
// file for the size ceiling, which less_view.go reached when the feed came in.

func runLess(ctx context.Context, screen tcell.Screen, feed *lessFeed, name string,
	settings lessOptions, stdout io.Writer) error {
	// The reader stops when the pager does, whether or not its input has ended.
	defer feed.stop()
	if err := screen.Init(); err != nil {
		return fmt.Errorf("cannot drive this terminal: %v", err)
	}
	defer screen.Fini()
	pager := &lessPager{name: name, settings: settings, screen: screen, feed: feed}
	pager.width, pager.height = screen.Size()
	if settings.quitIfOne {
		// -F: a text that fits needs no pager, so it is written out and left, which cannot be
		// known until a screenful more has arrived or the input has ended. The screen goes
		// first, so the text lands on a terminal that has been put back.
		if err := feed.waitFor(ctx, pager.textRows()); err != nil {
			return err
		}
		if pager.pull(); !pager.waiting && len(pager.lines) <= pager.textRows() {
			screen.Fini()
			return writeLessPlainly(stdout, pager.lines, settings)
		}
	}
	// A cancelled context has to interrupt PollEvent, which only an event will do. Until the
	// pager leaves: this waited on the context alone, and outlived every less whose context
	// was never cancelled.
	left := make(chan struct{})
	defer close(left)
	go func() {
		select {
		case <-ctx.Done():
			screen.PostEvent(tcell.NewEventInterrupt(nil))
		case <-left:
		}
	}()
	feed.attach(screen)
	for !pager.quit {
		pager.pull()
		pager.draw()
		event := screen.PollEvent()
		if event == nil {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		pager.handle(event)
	}
	return nil
}

// pull takes what the feed has read since, and whether it has ended. A read that failed says
// so on the bottom line, over what had been read before it.
func (p *lessPager) pull() {
	if p.feed == nil {
		return
	}
	lines, ended, err := p.feed.take()
	p.lines, p.waiting = lines, !ended
	if err != nil {
		p.message = err.Error()
	}
}
