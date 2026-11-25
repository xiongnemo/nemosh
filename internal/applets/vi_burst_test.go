package applets

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// queuedViScreen is a simulation screen whose keys are all waiting before vi reads the first,
// as a paste's are, and which counts the frames shown.
type queuedViScreen struct {
	tcell.SimulationScreen
	queue []tcell.Event
	shows int
}

func (s *queuedViScreen) PollEvent() tcell.Event {
	if len(s.queue) == 0 {
		return nil
	}
	event := s.queue[0]
	s.queue = s.queue[1:]
	return event
}

func (s *queuedViScreen) HasPendingEvent() bool { return len(s.queue) > 0 }

func (s *queuedViScreen) Show() {
	s.shows++
	s.SimulationScreen.Show()
}

// A paste is a key a character, and vi drew a frame for each: 4,580 of them took 9 seconds in
// conhost. Keys already waiting are handled first and drawn once after.
func TestVi_drawsOnceForKeysAlreadyWaiting(t *testing.T) {
	dir := viFixture(t, "one\n")
	editor := openVi(WithProcessView(context.Background(), hostProcessView{cwd: dir}), []string{"f.txt"}, false)
	pasted := strings.Repeat("pasted text\n", 20)
	screen := &queuedViScreen{SimulationScreen: tcell.NewSimulationScreen("UTF-8")}
	for _, character := range "i" + pasted {
		screen.queue = append(screen.queue, keyTypedAs(character))
	}
	screen.queue = append(screen.queue, tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone),
		keyTypedAs(':'), keyTypedAs('x'), keyTypedAs('\n'))
	if err := runVi(context.Background(), screen, editor); err != nil {
		t.Fatal(err)
	}
	if got := readViFile(t, filepath.Join(dir, "f.txt")); got != pasted+"one\n" {
		t.Fatalf("the file holds %q", got)
	}
	// The frame vi opens with, and none until the keys were all handled.
	if screen.shows > 2 {
		t.Fatalf("%d frames were drawn for %d keys", screen.shows, len(pasted)+5)
	}
}
