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
	for _, want := range []string{"top - up", "processes", "CPU", "Mem", "Cmt", "PID", "COMMAND"} {
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

	// Then -- pid 0, which is Idle, is on the *first row*. Asserted as the first row rather than
	// as "somewhere on screen", which is how this test used to be written: Idle is on screen in
	// the default CPU order too, so the weaker form passed whether or not the key did anything.
	deadline := time.Now().Add(10 * time.Second)
	last := ""
	for time.Now().Before(deadline) {
		cells, width, height := harness.cells(t)
		last = screenLine(cells, width, topHeaderRow(t, cells, width, height)+1)
		if strings.Contains(last, "Idle") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("sorting by pid did not put Idle on the first row; it holds %q", strings.TrimSpace(last))
}

// selectedPID is the pid text of the row under the cursor, read from tview's goroutine.
//
// The pid rather than the row number, because the row number is *supposed* to move: the list is
// sorted by CPU and reorders every second, and following the process rather than the row is the
// whole point of remembering a selection.
func (h *topHarness) selectedPID(t *testing.T) string {
	t.Helper()
	reply := make(chan string, 1)
	h.application.QueueUpdate(func() {
		table, ok := h.application.GetFocus().(*tview.Table)
		if !ok {
			reply <- ""
			return
		}
		row, _ := table.GetSelection()
		cell := table.GetCell(row, 0)
		if cell == nil {
			reply <- ""
			return
		}
		reply <- strings.TrimSpace(cell.Text)
	})
	select {
	case pid := <-reply:
		return pid
	case <-time.After(10 * time.Second):
		t.Fatal("the application stopped answering")
		return ""
	}
}

// cells reads the raw screen, styles included.
func (h *topHarness) cells(t *testing.T) ([]tcell.SimCell, int, int) {
	t.Helper()
	type contents struct {
		cells         []tcell.SimCell
		width, height int
	}
	reply := make(chan contents, 1)
	h.application.QueueUpdate(func() {
		cells, width, height := h.screen.GetContents()
		reply <- contents{cells: cells, width: width, height: height}
	})
	select {
	case got := <-reply:
		return got.cells, got.width, got.height
	case <-time.After(10 * time.Second):
		t.Fatal("the application stopped answering")
		return nil, 0, 0
	}
}

// The cursor stays on the process it was put on, across a refresh.
//
// The bug this pins was mine and it made the thing unusable: pressing Down moved the cursor and one
// second later it was back on the top row. Nothing to do with the key -- arrow keys are tview's and
// worked -- but the model only recorded the selection when some *other* key was pressed, so it sat
// at its zero value, and zero is a real pid here. Every refresh looked up pid 0, found Idle, and put
// the cursor obediently back on it.
func TestTopApplication_theCursorStaysOnTheProcessItWasPutOn(t *testing.T) {
	harness := startTop(t)
	defer harness.quit(t)
	if _, ok := harness.waitFor(t, "top - up"); !ok {
		t.Fatal("nothing drawn")
	}

	// When -- one row down. InjectKey is asynchronous and QueueUpdate is not ordered against it,
	// so the move is waited for rather than assumed. One press rather than three for the same
	// reason: with three, the poll below saw the selection change after the first had landed and
	// then reported the other two arriving as the cursor "moving on its own".
	before := harness.selectedPID(t)
	harness.screen.InjectKey(tcell.KeyDown, 0, tcell.ModNone)
	moved := ""
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if pid := harness.selectedPID(t); pid != "" && pid != before {
			moved = pid
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if moved == "" {
		t.Fatalf("the cursor never moved off pid %q", before)
	}

	// Then -- it is still on that process several refreshes later. With the bug it was back on
	// the top row within one.
	settle := time.Now().Add(3 * time.Second)
	for time.Now().Before(settle) {
		if now := harness.selectedPID(t); now != moved {
			t.Fatalf("the cursor moved from pid %s to pid %s on its own", moved, now)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// The table fills the window rather than sitting in a column on the left.
//
// Asserted through the header's background, which is the only way to see it: the drawn *text* is
// short either way, and what was wrong was the width of the columns underneath it. tview sizes a
// column to its widest cell, so without an expanding last column a table of short process names
// left two thirds of a wide terminal empty.
func TestTopApplication_theTableFillsTheWidth(t *testing.T) {
	harness := startTop(t)
	defer harness.quit(t)
	if _, ok := harness.waitFor(t, "COMMAND"); !ok {
		t.Fatal("nothing drawn")
	}

	// When
	cells, width, height := harness.cells(t)

	// Then -- the header row is styled to the last column
	header := topHeaderRow(t, cells, width, height)
	last := cells[header*width+width-1]
	_, background, _ := last.Style.Decompose()
	if background != tcell.ColorAqua && background != tcell.ColorGreen {
		t.Fatalf("the header stops short of the right edge: column %d has background %v",
			width-1, background)
	}
}

// The table is coloured, which is most of what makes four hundred rows readable.
func TestTopApplication_theTableIsColoured(t *testing.T) {
	harness := startTop(t)
	defer harness.quit(t)
	if _, ok := harness.waitFor(t, "COMMAND"); !ok {
		t.Fatal("nothing drawn")
	}

	// When
	cells, width, height := harness.cells(t)

	// Then -- something in the body of the table is drawn in a colour, and the meters above it
	// are too. A monochrome screen here means the styles were built and then dropped.
	foregrounds := map[tcell.Color]bool{}
	for row := 0; row < height; row++ {
		for column := 0; column < width; column++ {
			foreground, _, _ := cells[row*width+column].Style.Decompose()
			if foreground != tcell.ColorDefault {
				foregrounds[foreground] = true
			}
		}
	}
	if len(foregrounds) < 3 {
		t.Fatalf("the screen uses %d colours, want a scheme: %v", len(foregrounds), foregrounds)
	}
}

// topHeaderRow finds the table's header line on the screen.
//
// Searched for rather than computed from a constant: the header's height follows the machine now,
// since every processor gets a meter, so a test that assumed a fixed offset would be asserting
// against the wrong row on a machine with a different number of cores.
func topHeaderRow(t *testing.T, cells []tcell.SimCell, width, height int) int {
	t.Helper()
	for row := 0; row < height; row++ {
		if strings.Contains(screenLine(cells, width, row), "COMMAND") {
			return row
		}
	}
	t.Fatal("no header row on screen")
	return 0
}

// screenLine is one row of the simulated screen as text.
func screenLine(cells []tcell.SimCell, width, row int) string {
	line := ""
	for column := 0; column < width; column++ {
		if runes := cells[row*width+column].Runes; len(runes) > 0 {
			line += string(runes[0])
		}
	}
	return line
}
