package applets

import (
	"context"
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
)

// The pager itself: what is on the screen, and what a key does to it.
//
// **`top` is clamped so the last screenful is the last screenful**, which is the rule that
// makes `G` and a page-forward at the end agree with each other. Without it a reader can
// scroll past the text into blank rows and then wonder whether the file really ended.

type lessPager struct {
	lines    []string
	name     string
	settings lessOptions
	// top is the index of the first line on the screen.
	top int
	// message is the bottom line: the prompt, or what a failed search had to say.
	message string
	// pattern is the last search, which `n` and `N` repeat.
	pattern string
	screen  tcell.Screen
	width   int
	height  int
	// pending is the search being typed, and searching says which direction it is.
	pending   string
	searching byte
	quit      bool
}

func runLess(ctx context.Context, screen tcell.Screen, lines []string, name string,
	settings lessOptions, stdout io.Writer) error {
	if err := screen.Init(); err != nil {
		return fmt.Errorf("cannot drive this terminal: %v", err)
	}
	defer screen.Fini()
	pager := &lessPager{lines: lines, name: name, settings: settings, screen: screen}
	pager.width, pager.height = screen.Size()
	if settings.quitIfOne && len(lines) <= pager.textRows() {
		// -F: a file that fits needs no pager, so it is written out and left. The
		// screen goes first, so the text lands on a terminal that has been put back.
		screen.Fini()
		return writeLessPlainly(stdout, lines, settings)
	}
	go func() {
		<-ctx.Done()
		// A cancelled context has to interrupt PollEvent, which only an event will do.
		screen.PostEvent(tcell.NewEventInterrupt(nil))
	}()
	for !pager.quit {
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

// textRows is how many rows hold text: everything but the prompt at the bottom.
func (p *lessPager) textRows() int {
	if p.height < 2 {
		return 1
	}
	return p.height - 1
}

// clampTop keeps the view inside the text.
//
// The last screenful is the last screenful: `top` never goes past the point where the final
// line sits on the bottom row, so paging forward at the end does nothing rather than
// scrolling into blank space.
func (p *lessPager) clampTop() {
	highest := len(p.lines) - p.textRows()
	if highest < 0 {
		highest = 0
	}
	if p.top > highest {
		p.top = highest
	}
	if p.top < 0 {
		p.top = 0
	}
}

func (p *lessPager) draw() {
	p.clampTop()
	p.screen.Clear()
	style := tcell.StyleDefault
	for row := 0; row < p.textRows(); row++ {
		index := p.top + row
		if index >= len(p.lines) {
			// Past the end less draws a tilde, which is how a reader tells "the file
			// ended" from "the screen is short".
			p.drawText(0, row, "~", style.Foreground(tcell.ColorBlue))
			continue
		}
		prefix := ""
		if p.settings.numbers {
			prefix = fmt.Sprintf("%6d  ", index+1)
		}
		text := prefix + p.lines[index]
		if p.settings.chop && len(text) > p.width {
			// -S: long lines are cut rather than folded, so every row is one line.
			text = text[:p.width]
		}
		// The spans are found in the line itself and then shifted past the line number,
		// so `-N` moves the highlight without changing what matched.
		p.drawMatched(row, text, style, p.matchSpans(p.lines[index], utf8.RuneCountInString(prefix)))
	}
	p.drawText(0, p.height-1, p.prompt(), style.Reverse(true))
	p.screen.Show()
}

func (p *lessPager) drawText(column, row int, text string, style tcell.Style) {
	for _, character := range text {
		if column >= p.width {
			return
		}
		p.screen.SetContent(column, row, character, nil, style)
		column++
	}
}

// prompt is the bottom line: what is being typed, what went wrong, or where we are.
func (p *lessPager) prompt() string {
	if p.searching != 0 {
		return string(p.searching) + p.pending
	}
	if p.message != "" {
		return p.message
	}
	if p.atEnd() {
		return fmt.Sprintf("%s (END) -- press q to quit", p.name)
	}
	if len(p.lines) == 0 {
		return p.name + " (empty)"
	}
	last := p.top + p.textRows()
	if last > len(p.lines) {
		last = len(p.lines)
	}
	return fmt.Sprintf("%s lines %d-%d/%d %d%%", p.name, p.top+1, last, len(p.lines),
		last*100/len(p.lines))
}

func (p *lessPager) atEnd() bool {
	return len(p.lines) == 0 || p.top+p.textRows() >= len(p.lines)
}

func (p *lessPager) handle(event tcell.Event) {
	switch typed := event.(type) {
	case *tcell.EventResize:
		p.width, p.height = typed.Size()
		p.screen.Sync()
	case *tcell.EventKey:
		if p.searching != 0 {
			p.typeSearch(typed)
			return
		}
		p.command(typed)
	}
}

// typeSearch collects the pattern after a `/` or `?`.
func (p *lessPager) typeSearch(event *tcell.EventKey) {
	switch event.Key() {
	case tcell.KeyEnter:
		direction := p.searching
		p.searching = 0
		if p.pending != "" {
			p.pattern = p.pending
		}
		p.pending = ""
		p.runSearch(direction == '/', p.top+1)
	case tcell.KeyEscape, tcell.KeyCtrlC:
		p.searching, p.pending = 0, ""
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if p.pending != "" {
			p.pending = p.pending[:len(p.pending)-1]
		}
	case tcell.KeyRune:
		p.pending += string(event.Rune())
	}
}

func (p *lessPager) command(event *tcell.EventKey) {
	p.message = ""
	switch event.Key() {
	case tcell.KeyDown, tcell.KeyEnter:
		p.top++
	case tcell.KeyUp:
		p.top--
	case tcell.KeyPgDn:
		p.top += p.textRows()
	case tcell.KeyPgUp:
		p.top -= p.textRows()
	case tcell.KeyHome:
		p.top = 0
	case tcell.KeyEnd:
		p.top = len(p.lines)
	case tcell.KeyCtrlC:
		p.quit = true
	case tcell.KeyRune:
		p.runeCommand(event.Rune())
	}
	p.clampTop()
}

func (p *lessPager) runeCommand(key rune) {
	switch key {
	case 'q', 'Q':
		p.quit = true
	case ' ', 'f':
		p.top += p.textRows()
		if p.settings.quitAtEOF && p.atEnd() {
			// -E: a page forward at the end leaves, rather than sitting on (END).
			p.quit = true
		}
	case 'b':
		p.top -= p.textRows()
	case 'j':
		p.top++
	case 'k':
		p.top--
	case 'd':
		p.top += p.textRows() / 2
	case 'u':
		p.top -= p.textRows() / 2
	case 'g':
		p.top = 0
	case 'G':
		p.top = len(p.lines)
	case '/', '?':
		p.searching, p.pending = byte(key), ""
	case 'n':
		p.runSearch(true, p.top+1)
	case 'N':
		p.runSearch(false, p.top-1)
	}
}

// runSearch moves to the next line matching the pattern, in the given direction.
//
// It does **not** wrap, and says so when it runs out: a pager that wrapped would take a
// reader back to the top without their noticing, and the message is the only way to tell
// "no more" from "none at all".
func (p *lessPager) runSearch(forward bool, from int) {
	if p.pattern == "" {
		p.message = "no previous pattern"
		return
	}
	compiled, ok := p.compiledPattern()
	if !ok {
		p.message = "bad pattern: " + p.pattern
		return
	}
	step := 1
	if !forward {
		step = -1
	}
	for index := from; index >= 0 && index < len(p.lines); index += step {
		if compiled.MatchString(p.lines[index]) {
			p.top = index
			p.clampTop()
			return
		}
	}
	p.message = "pattern not found: " + p.pattern
}

// lessSearchFrom is the search on its own, so a test can check where it lands without a
// screen. It answers the line it found, or -1.
func lessSearchFrom(lines []string, pattern string, from int, forward, ignoreCase bool) int {
	pager := &lessPager{lines: lines, pattern: pattern, height: 2}
	pager.settings.ignoreCase = ignoreCase
	pager.runSearch(forward, from)
	if pager.message != "" {
		return -1
	}
	return pager.top
}
