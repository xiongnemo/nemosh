package applets

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

// scriptedScreen hands out its events in order, each after its delay, and then nil, as a
// screen that has been finished does.
type scriptedScreen struct {
	tcell.Screen
	events []scriptedEvent
}

type scriptedEvent struct {
	after time.Duration
	event tcell.Event
}

func (s *scriptedScreen) PollEvent() tcell.Event {
	if len(s.events) == 0 {
		return nil
	}
	next := s.events[0]
	s.events = s.events[1:]
	time.Sleep(next.after)
	return next.event
}

// typed is text as the keys a console delivers for it, with no time between them.
func typed(text string) []scriptedEvent {
	var events []scriptedEvent
	for _, character := range text {
		events = append(events, scriptedEvent{event: keyTypedAs(character)})
	}
	return events
}

// describeBurstEvents names what came out: `{` and `}` a paste's two ends, a typed character
// as itself, and any other key by tcell's name for it.
func describeBurstEvents(screen tcell.Screen) string {
	var names []string
	for {
		event := screen.PollEvent()
		if event == nil {
			return strings.Join(names, " ")
		}
		switch event := event.(type) {
		case *tcell.EventPaste:
			names = append(names, map[bool]string{true: "{", false: "}"}[event.Start()])
		case *tcell.EventKey:
			if event.Key() == tcell.KeyRune && event.Modifiers()&^tcell.ModShift == 0 {
				names = append(names, string(event.Rune()))
			} else {
				names = append(names, event.Name())
			}
		}
	}
}

func TestTypingBursts_tellTypedKeysThatArriveTogetherAsAPaste(t *testing.T) {
	const gap = 100 * time.Millisecond
	pause := scriptedEvent{after: 4 * gap, event: keyTypedAs('c')}
	shifted := scriptedEvent{event: tcell.NewEventKey(tcell.KeyRune, 'A', tcell.ModShift)}
	alt := scriptedEvent{event: tcell.NewEventKey(tcell.KeyRune, 'f', tcell.ModAlt)}
	ctrlO := scriptedEvent{event: tcell.NewEventKey(tcell.KeyCtrlO, 0, tcell.ModCtrl)}
	cases := []struct {
		name   string
		events []scriptedEvent
		want   string
	}{
		{"one key is a key", typed("a"), "a"},
		{"keys together are a paste, Enter and Tab with them", typed("ab\n\tc"), "{ a b Enter Tab c }"},
		{"a binding ends the paste and is a key", append(typed("ab"), ctrlO), "{ a b } Ctrl+O"},
		{"a pause ends it", append(append(typed("ab"), pause), typed("d")...), "{ a b } { c d }"},
		{"Shift is typing", append([]scriptedEvent{shifted}, typed("b")...), "{ A b }"},
		{"Alt is not", append([]scriptedEvent{alt}, typed("g")...), "Alt+Rune[f] g"},
		{"a paste the terminal bracketed passes as it came", append(append(
			[]scriptedEvent{{event: tcell.NewEventPaste(true)}}, typed("ab")...),
			scriptedEvent{event: tcell.NewEventPaste(false)}), "{ a b }"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			screen := newTypingBursts(&scriptedScreen{events: c.events})
			screen.gap = gap
			if got := describeBurstEvents(screen); got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

// postPaste sends text as a paste the terminal bracketed. The simulation screen's queue
// holds ten events and PostEvent does not wait, so a full queue is waited out.
func (h *editorHarness) postPaste(t *testing.T, text string) {
	t.Helper()
	post := func(event tcell.Event) {
		for {
			err := h.screen.PostEvent(event)
			if !errors.Is(err, tcell.ErrEventQFull) {
				return
			}
			time.Sleep(time.Millisecond)
		}
	}
	post(tcell.NewEventPaste(true))
	for _, character := range text {
		post(keyTypedAs(character))
	}
	post(tcell.NewEventPaste(false))
}

func (h *editorHarness) area(t *testing.T) *highlightedArea {
	t.Helper()
	reply := make(chan *highlightedArea, 1)
	go h.application.QueueUpdate(func() { reply <- h.application.GetFocus().(*highlightedArea) })
	select {
	case area := <-reply:
		return area
	case <-time.After(10 * time.Second):
		t.Fatal("the editor stopped answering")
		return nil
	}
}

func TestEditor_takesAPasteAsTypedAndLexesItOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "paste.py")
	harness := startEditor(t, "nano", path)
	defer harness.stop(t)
	var block strings.Builder
	for line := range 60 {
		block.WriteString("def f" + strconv.Itoa(line) + "(value):  # note\n")
	}
	area := harness.area(t)
	before := harness.lexes(t, area)
	harness.postPaste(t, block.String())
	harness.waitForScreen(t, "def f59(value)")
	text := make(chan string, 1)
	go harness.application.QueueUpdate(func() { text <- area.GetText() })
	if got := <-text; got != block.String() {
		t.Fatalf("the buffer holds %q, want the paste", got)
	}
	// Once for the frame the paste is drawn in. A lex a key was 60 lines lexed 1,500 times.
	if lexes := harness.lexes(t, area) - before; lexes > 2 {
		t.Fatalf("the paste was lexed %d times", lexes)
	}
}

func (h *editorHarness) lexes(t *testing.T, area *highlightedArea) int {
	t.Helper()
	reply := make(chan int, 1)
	go h.application.QueueUpdate(func() { reply <- area.lexes })
	return <-reply
}

func TestEditor_aPasteIntoAPromptAnswersIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "haystack.txt")
	var lines strings.Builder
	for line := 1; line <= 50; line++ {
		lines.WriteString("number " + strings.Repeat("-", line%3) + strconv.Itoa(line) + "\n")
	}
	if err := os.WriteFile(path, []byte(lines.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	harness := startEditor(t, "nano", path)
	defer harness.stop(t)
	harness.press(t, tcell.KeyCtrlW, 0)
	harness.waitForScreen(t, "Search")
	// The newline is Enter, which answers the prompt, as it would typed.
	harness.postPaste(t, "number -37\n")
	harness.waitForScreen(t, "Found on line 37")
}
