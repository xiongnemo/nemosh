package applets

import (
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Pasting into the editor, which a console delivers as keys.
//
// A paste reaches the editor as one key event a character, and tview draws the whole screen
// after every key event. A hundred lines of Python, 4,580 characters, took 21 seconds to
// appear in conhost, measured 2026-11-26: a frame for every character, each written out to
// the console, and the buffer lexed whole for every one besides. tview takes key events
// without drawing in one case only, a paste, which it collects and hands over whole. So
// typed keys that arrive together are told to tview as a paste -- nobody types two keys
// within a few milliseconds, and a paste delivers thousands that way -- and the editor's
// root replays it as the keys it was (typedAsKeys). Nothing behaves differently but the
// drawing: a prompt still takes the keys as an answer, and Enter in them still answers it.
//
// The terminal is not asked for bracketed paste: finding the burst works in a console that
// cannot bracket one as well. (This said once that the shell's line editor does not read the
// markers. It does, and turns the mode on for each line it reads and off after;
// cmd/nemosh's lineedit_paste_test.go.)

// typingBurstGap is how long after a typed key the next must come to be in the same burst.
// It is also how long a key typed alone waits before it is handled, which is why it is short.
const typingBurstGap = 5 * time.Millisecond

// typingBursts is a screen that reports typed keys arriving together as a paste.
type typingBursts struct {
	tcell.Screen
	gap time.Duration
	// events are the screen's, read on a goroutine of their own so that the next one can be
	// waited for with a limit.
	events chan tcell.Event
	start  sync.Once
	// held are events already read and not yet returned, in order.
	held []tcell.Event
	// inBurst is a paste this screen opened and has not closed; bracketed, one the terminal
	// did, which is passed through as it came.
	inBurst, bracketed bool
}

func newTypingBursts(screen tcell.Screen) *typingBursts {
	return &typingBursts{Screen: screen, gap: typingBurstGap, events: make(chan tcell.Event)}
}

// EnablePaste is refused; see the file comment. tview is told paste is on, so that it takes
// the bursts this screen reports.
func (s *typingBursts) EnablePaste() {}

func (s *typingBursts) PollEvent() tcell.Event {
	s.start.Do(func() { go s.pump() })
	event, ok := s.next(s.inBurst)
	switch {
	case !ok:
		s.inBurst = false
		return tcell.NewEventPaste(false)
	case event == nil && s.inBurst:
		return s.endBurst(nil)
	case event == nil:
		return nil
	}
	if paste, isPaste := event.(*tcell.EventPaste); isPaste {
		if s.inBurst {
			return s.endBurst(event)
		}
		s.bracketed = paste.Start()
		return event
	}
	typed := isTypedKey(event)
	switch {
	case s.bracketed || s.inBurst && typed:
		return event
	case s.inBurst:
		return s.endBurst(event)
	case !typed:
		return event
	}
	// A typed key alone. Whether a burst starts is whether the next event comes soon and is
	// typed too.
	following, soon := s.next(true)
	if !soon {
		return event
	}
	if !isTypedKey(following) {
		s.held = append([]tcell.Event{following}, s.held...)
		return event
	}
	s.held = append([]tcell.Event{event, following}, s.held...)
	s.inBurst = true
	return tcell.NewEventPaste(true)
}

// next is the next event, a held one first. With soon, it waits only the gap, and answers
// false if nothing came in it.
func (s *typingBursts) next(soon bool) (tcell.Event, bool) {
	if len(s.held) > 0 {
		event := s.held[0]
		s.held = s.held[1:]
		return event, true
	}
	if !soon {
		return <-s.events, true
	}
	timer := time.NewTimer(s.gap)
	defer timer.Stop()
	select {
	case event := <-s.events:
		return event, true
	case <-timer.C:
		return nil, false
	}
}

// endBurst closes the burst, keeping the event that ended it for next time.
func (s *typingBursts) endBurst(event tcell.Event) tcell.Event {
	s.inBurst = false
	s.held = append([]tcell.Event{event}, s.held...)
	return tcell.NewEventPaste(false)
}

// pump reads the screen's events until the screen is finished, which it says with nil.
func (s *typingBursts) pump() {
	for {
		event := s.Screen.PollEvent()
		s.events <- event
		if event == nil {
			return
		}
	}
}

// isTypedKey is a key that types a character where the text is: a character, with Shift or
// without, Enter, or Tab. tview collects these three into a paste and drops any other key.
func isTypedKey(event tcell.Event) bool {
	key, ok := event.(*tcell.EventKey)
	if !ok {
		return false
	}
	switch key.Key() {
	case tcell.KeyRune:
		return key.Modifiers()&^tcell.ModShift == 0
	case tcell.KeyEnter, tcell.KeyTab:
		return key.Modifiers() == 0
	}
	return false
}

// keyTypedAs is the key that types a character, the inverse of how tview collected it.
func keyTypedAs(character rune) *tcell.EventKey {
	switch character {
	case '\n':
		return tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
	case '\t':
		return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
	}
	return tcell.NewEventKey(tcell.KeyRune, character, tcell.ModNone)
}

// typedAsKeys is the editor's root, which takes a paste as the keys it was typed as: each goes
// where a key goes, through the input capture that decides whether it is text, an answer to a
// prompt, or refused by a read-only buffer.
type typedAsKeys struct{ tview.Primitive }

func (r typedAsKeys) PasteHandler() func(string, func(tview.Primitive)) {
	return func(text string, setFocus func(tview.Primitive)) {
		handler := r.InputHandler()
		if handler == nil {
			return
		}
		for _, character := range text {
			handler(keyTypedAs(character), setFocus)
		}
	}
}
