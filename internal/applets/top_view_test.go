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
		drawn := harness.draw(t)
		last = drawn.line(drawn.headerRow(t) + 1)
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

// screen reads the drawn screen, styles included.
//
// A copy, taken inside the queued call, and that is the whole point of the type. SimulationScreen's
// GetContents returns the *live* buffer the application draws into, so handing that slice back to
// the test goroutine races every redraw -- which the race detector duly found once a test started
// polling it in a loop. The file's own note at the top says the screen may only be read from tview's
// goroutine; returning a slice from there is still reading it from here.
type drawnScreen struct {
	width, height int
	lines         []string
	styles        []tcell.Style
}

// line is one row of the screen as text.
func (d drawnScreen) line(row int) string {
	if row < 0 || row >= len(d.lines) {
		return ""
	}
	return d.lines[row]
}

// style is one cell's style.
func (d drawnScreen) style(row, column int) tcell.Style {
	if row < 0 || row >= d.height || column < 0 || column >= d.width {
		return tcell.StyleDefault
	}
	return d.styles[row*d.width+column]
}

// headerRow is where the table's header landed.
//
// Searched for rather than computed: the header's height follows the machine, since every processor
// gets a meter, so a fixed offset would point at the wrong row on a machine with a different number
// of cores.
func (d drawnScreen) headerRow(t *testing.T) int {
	t.Helper()
	for row := 0; row < d.height; row++ {
		if strings.Contains(d.line(row), "COMMAND") {
			return row
		}
	}
	t.Fatalf("no header row on screen:\n%s", strings.Join(d.lines, "\n"))
	return 0
}

func (h *topHarness) draw(t *testing.T) drawnScreen {
	t.Helper()
	reply := make(chan drawnScreen, 1)
	h.application.QueueUpdate(func() {
		cells, width, height := h.screen.GetContents()
		drawn := drawnScreen{
			width: width, height: height,
			lines:  make([]string, 0, height),
			styles: make([]tcell.Style, width*height),
		}
		for row := 0; row < height; row++ {
			var line strings.Builder
			for column := 0; column < width; column++ {
				cell := cells[row*width+column]
				drawn.styles[row*width+column] = cell.Style
				if len(cell.Runes) == 0 {
					line.WriteByte(' ')
					continue
				}
				line.WriteRune(cell.Runes[0])
			}
			drawn.lines = append(drawn.lines, line.String())
		}
		reply <- drawn
	})
	select {
	case drawn := <-reply:
		return drawn
	case <-time.After(10 * time.Second):
		t.Fatal("the application stopped answering")
		return drawnScreen{}
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
	drawn := harness.draw(t)

	// Then -- the header row is styled to the last column
	_, background, _ := drawn.style(drawn.headerRow(t), drawn.width-1).Decompose()
	if background != tcell.ColorAqua && background != tcell.ColorGreen {
		t.Fatalf("the header stops short of the right edge: column %d has background %v",
			drawn.width-1, background)
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
	drawn := harness.draw(t)

	// Then -- something in the body of the table is drawn in a colour, and the meters above it
	// are too. A monochrome screen here means the styles were built and then dropped.
	foregrounds := map[tcell.Color]bool{}
	for row := 0; row < drawn.height; row++ {
		for column := 0; column < drawn.width; column++ {
			foreground, _, _ := drawn.style(row, column).Decompose()
			if foreground != tcell.ColorDefault {
				foregrounds[foreground] = true
			}
		}
	}
	if len(foregrounds) < 3 {
		t.Fatalf("the screen uses %d colours, want a scheme: %v", len(foregrounds), foregrounds)
	}
}

// The columns do not move while the table is being read.
//
// This is the one that made the table hard to read, and it is not obvious from the code: tview sizes
// a column to the widest cell *among the rows it can see*, so scrolling past a process with a long
// name widened that column and shifted everything to its right, and scrolling back shifted them
// back. Nothing was wrong with any single frame. The fix is that every column is padded to its
// declared width and clamped to it, so what is on screen cannot change the layout.
func TestTopApplication_theColumnsDoNotMoveWhileScrolling(t *testing.T) {
	harness := startTop(t)
	defer harness.quit(t)
	if _, ok := harness.waitFor(t, "COMMAND"); !ok {
		t.Fatal("nothing drawn")
	}
	drawn := harness.draw(t)
	before := drawn.line(drawn.headerRow(t))

	// When -- forty rows down, well past a screenful, through several refreshes
	for range 40 {
		harness.screen.InjectKey(tcell.KeyDown, 0, tcell.ModNone)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		drawn = harness.draw(t)
		after := drawn.line(drawn.headerRow(t))

		// Then -- the header is in exactly the same place, which it can only be if every
		// column is still the same width
		if after != before {
			t.Fatalf("the columns moved while scrolling:\nbefore %q\nafter  %q", before, after)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
