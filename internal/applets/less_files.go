package applets

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/gdamore/tcell/v2"
)

// The FILEs less pages, one at a time, as GNU's less pages them: `:n` is the next, `:p` the one
// before, `:x` the first, and `:q` leaves. Each is read by a feed of its own, made when it is
// first shown and kept, so a FILE gone back to is where it was, standard input included, and the
// prompt says which of how many it is. They were one stream, the FILEs one after another, so
// `less a b` had no way to tell where a ended. With nowhere to page to they are still one after
// another, as cat would write them.

// lessFile is one FILE: where it comes from, its name, what has been read of it, and where the
// screen was in it.
type lessFile struct {
	source lessSource
	name   string
	feed   *lessFeed
	top    int
}

// openLessFiles opens what is to be read, the operands or standard input, before anything is
// read: a FILE that cannot be opened stops less with the reason rather than turning up partway
// through the paging.
func openLessFiles(ctx context.Context, operands []string, stdin io.Reader) ([]*lessFile, error) {
	if len(operands) == 0 {
		return []*lessFile{{source: lessSource{reader: stdin}, name: "(standard input)"}}, nil
	}
	view := ProcessViewFromContext(ctx)
	var files []*lessFile
	for _, operand := range operands {
		if operand == "-" {
			files = append(files, &lessFile{source: lessSource{reader: stdin}, name: "(standard input)"})
			continue
		}
		native, err := resolveHostPath(view, operand)
		if err == nil {
			var file *os.File
			if file, err = os.Open(native); err == nil {
				files = append(files, &lessFile{source: lessSource{reader: file, close: func() { file.Close() }, native: native}, name: operand})
				continue
			}
			err = cannotOpen(operand, err)
		}
		for _, opened := range files {
			if opened.source.close != nil {
				opened.source.close()
			}
		}
		return nil, err
	}
	return files, nil
}

// switchTo shows the FILE at index, reading it if it has not been read, where it was left.
// Following ends with the FILE that was followed.
func (p *lessPager) switchTo(index int) {
	if p.following {
		p.stopFollowing()
	}
	if p.current < len(p.files) && p.files[p.current].feed != nil {
		p.files[p.current].top = p.top
	}
	p.current = index
	file := p.files[index]
	if file.feed == nil {
		file.feed = startLessFeed(p.ctx, []lessSource{file.source})
		if p.attached {
			file.feed.attach(p.screen)
		}
	}
	p.feed, p.top, p.name = file.feed, file.top, file.name
	if len(p.files) > 1 {
		p.name = fmt.Sprintf("%s (file %d of %d)", file.name, index+1, len(p.files))
	}
	p.pull()
}

// colonCommand is the key after `:`.
func (p *lessPager) colonCommand(event *tcell.EventKey) {
	switch event.Rune() {
	case 'n':
		if p.current+1 >= len(p.files) {
			p.message = "No next file"
			return
		}
		p.switchTo(p.current + 1)
	case 'p':
		if p.current == 0 || len(p.files) == 0 {
			p.message = "No previous file"
			return
		}
		p.switchTo(p.current - 1)
	case 'x':
		if len(p.files) > 0 {
			p.switchTo(0)
		}
	case 'q', 'Q':
		p.quit = true
	default:
		p.message = fmt.Sprintf("no command :%c here; :n, :p, :x and :q are", event.Rune())
	}
}

// stopFeeds ends every FILE's reading, as less leaves.
func (p *lessPager) stopFeeds() {
	for _, file := range p.files {
		if file.feed != nil {
			file.feed.stop()
		}
	}
	for _, file := range p.files {
		// A FILE never shown was never read, and is let go of here.
		if file.feed == nil && file.source.close != nil {
			file.source.close()
		}
	}
}

// lessSources are the FILEs' inputs one after another, for the cat path.
func lessSources(files []*lessFile) []lessSource {
	sources := make([]lessSource, 0, len(files))
	for _, file := range files {
		sources = append(sources, file.source)
	}
	return sources
}
