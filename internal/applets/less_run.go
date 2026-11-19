package applets

import (
	"context"
	"fmt"
	"io"

	"github.com/gdamore/tcell/v2"
)

// Running the pager: the screen, the loop, and what the feed has read; see less_feed.go. Its own
// file for the size ceiling, which less_view.go reached when the feed came in.

func runLess(ctx context.Context, screen tcell.Screen, files []*lessFile, start lessStart,
	settings lessOptions, stdout io.Writer) error {
	pager := &lessPager{ctx: ctx, files: files, start: start, settings: settings, screen: screen}
	// The readers stop when the pager does, whether or not their input has ended.
	defer pager.stopFeeds()
	if err := screen.Init(); err != nil {
		return fmt.Errorf("cannot drive this terminal: %v", err)
	}
	defer screen.Fini()
	pager.width, pager.height = screen.Size()
	pager.switchTo(0)
	feed := pager.feed
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
	pager.attached = true
	for !pager.quit {
		pager.pull()
		pager.settle()
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
	if p.following {
		// F keeps the last line on the screen as lines come.
		p.top = len(p.lines)
	}
}
