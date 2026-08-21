package applets

import (
	"context"
	"io"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// The drawn form, driven headlessly.
//
// tcell has a simulation screen, which means the interactive path is testable after all -- the
// application runs, the widgets fill, keys can be injected and the cells read back. That is worth
// more than it sounds: without it the only check on the drawn form is a person looking at a
// terminal, and "it did not draw" is then indistinguishable from "it drew the wrong thing".
//
// The screen may only be read from tview's own goroutine, which is what the harness is for.
// Reading it from the test goroutine races the drawing: the race detector caught tview's SetScreen
// calling the screen's Init while this side read cells, and that is inherent to polling a screen
// the application owns rather than something a sleep would fix. Going through QueueUpdate also
// solves the startup problem for free, because the call is not received until the application is
// running.

// topHarness is a running application and the means to look at it.
type topHarness struct {
	application *tview.Application
	screen      tcell.SimulationScreen
	finished    chan error
}

func startTop(t *testing.T) *topHarness {
	t.Helper()
	if runtime.GOOS != "windows" {
		// internal/proc samples on Windows only and refuses elsewhere rather than
		// guessing, so the application would draw a sampling failure and nothing else --
		// correct behaviour, and not what these tests are about. TestPs takes the same
		// guard for the same reason.
		t.Skip("the process table is implemented on Windows only")
	}
	// Handed over uninitialised: tview's SetScreen calls Init, and calling it here as well
	// makes two writers to the screen's buffers. The default 80x25 fits everything asserted.
	screen := tcell.NewSimulationScreen("UTF-8")
	options, err := topArgs(nil)
	if err != nil {
		t.Fatalf("topArgs: %v", err)
	}
	harness := &topHarness{screen: screen, finished: make(chan error, 1)}
	ready := make(chan *tview.Application, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	go func() { harness.finished <- runTopApplication(ctx, options, screen, io.Discard, ready) }()
	select {
	case harness.application = <-ready:
	case <-time.After(10 * time.Second):
		t.Fatal("the application never started")
	}
	return harness
}

// text reads the screen from tview's goroutine.
func (h *topHarness) text(t *testing.T) string {
	t.Helper()
	reply := make(chan string, 1)
	h.application.QueueUpdate(func() { reply <- simulationText(h.screen) })
	select {
	case text := <-reply:
		return text
	case <-time.After(10 * time.Second):
		t.Fatal("the application stopped answering")
		return ""
	}
}

func simulationText(screen tcell.SimulationScreen) string {
	cells, width, height := screen.GetContents()
	var out strings.Builder
	for row := 0; row < height; row++ {
		for column := 0; column < width; column++ {
			runes := cells[row*width+column].Runes
			if len(runes) == 0 {
				out.WriteByte(' ')
				continue
			}
			out.WriteRune(runes[0])
		}
		out.WriteByte('\n')
	}
	return out.String()
}

// waitFor polls until the screen shows want, answering what it last saw either way.
func (h *topHarness) waitFor(t *testing.T, want string) (string, bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	last := ""
	for time.Now().Before(deadline) {
		last = h.text(t)
		if strings.Contains(last, want) {
			return last, true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return last, false
}

func (h *topHarness) quit(t *testing.T) error {
	t.Helper()
	h.screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	select {
	case err := <-h.finished:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("q did not quit")
		return nil
	}
}

// The whole drawn form, end to end: it starts, samples the real machine, fills a table, and quits.
func TestTopApplication_drawsAndQuits(t *testing.T) {
	harness := startTop(t)

	// When
	drawn, ok := harness.waitFor(t, "top - up")

	// Then
	if !ok {
		t.Fatalf("nothing was drawn; screen was:\n%s", drawn)
	}
	for _, want := range []string{"top - up", "processes", "Mem:", "PID", "COMMAND"} {
		if !strings.Contains(drawn, want) {
			t.Fatalf("screen does not mention %q:\n%s", want, drawn)
		}
	}
	if !strings.Contains(drawn, "System") && !strings.Contains(drawn, "Idle") {
		t.Fatalf("no recognisable process on screen:\n%s", drawn)
	}
	// The key hints are what make the thing discoverable at all.
	if !strings.Contains(drawn, "quit") {
		t.Fatalf("no key hints on screen:\n%s", drawn)
	}

	// And q is the one key that must always work.
	if err := harness.quit(t); err != nil {
		t.Fatalf("application returned %v", err)
	}
}

// A sort key changes what is on screen, which is the whole of what the drawn form adds over the
// plain one.
func TestTopApplication_sortKeyReordersTheTable(t *testing.T) {
	harness := startTop(t)
	defer harness.quit(t)
	if _, ok := harness.waitFor(t, "top - up"); !ok {
		t.Fatal("nothing drawn")
	}

	// When -- 1 sorts by the first column, the pid in the default layout
	harness.screen.InjectKey(tcell.KeyRune, '1', tcell.ModNone)

	// Then -- ascending pid brings pid 0, which is Idle, to the top
	if screen, ok := harness.waitFor(t, "Idle"); !ok {
		t.Fatalf("sorting by pid did not bring Idle into view:\n%s", screen)
	}
}
