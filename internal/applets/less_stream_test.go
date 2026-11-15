package applets

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

// less draws what it has while more is coming, as busybox's reads only what the screen needs:
// it read all of its input before drawing anything, so `tail -f log | less` never drew and
// `make | less` drew when make was done. And it leaves its reader stopped behind it.

// pagerOnPipe starts the pager on a pipe that stays open until the test writes and closes it.
func pagerOnPipe(t *testing.T, settings lessOptions) (tcell.SimulationScreen, *os.File, *lessFeed, chan error) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { writer.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	screen := tcell.NewSimulationScreen("UTF-8")
	feed := startLessFeed(ctx, []lessSource{{reader: reader, close: func() { reader.Close() }}}, settings.raw)
	finished := make(chan error, 1)
	go func() { finished <- runLess(ctx, screen, feed, "(standard input)", settings, io.Discard) }()
	// Not read until runLess has started it: the simulation screen's Init is not locked
	// against its Size. attach comes after Init, under the feed's lock, which orders the two.
	deadline := time.Now().Add(5 * time.Second)
	for {
		feed.mu.Lock()
		attached := feed.screen != nil
		feed.mu.Unlock()
		if attached {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the pager never started")
		}
		time.Sleep(5 * time.Millisecond)
	}
	return screen, writer, feed, finished
}

// screenText is the screen's rows, joined, read while the pager may be drawing: the
// simulation screen locks its own cells.
func screenText(screen tcell.SimulationScreen) string {
	width, height := screen.Size()
	var out strings.Builder
	for row := 0; row < height; row++ {
		for column := 0; column < width; column++ {
			text, _, _ := screen.Get(column, row)
			if text == "" {
				text = " "
			}
			out.WriteString(text)
		}
		out.WriteString("\n")
	}
	return out.String()
}

func waitForScreenText(t *testing.T, screen tcell.SimulationScreen, want string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	last := ""
	for time.Now().Before(deadline) {
		if last = screenText(screen); strings.Contains(last, want) {
			return last
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("the screen never showed %q; it showed:\n%s", want, last)
	return ""
}

func TestLess_drawsWhatHasArrivedBeforeItsInputEnds(t *testing.T) {
	screen, writer, _, finished := pagerOnPipe(t, lessOptions{})

	// When: one line, and the input still open
	fmt.Fprintln(writer, "the first line")

	// Then: drawn, and not said to be the end
	frame := waitForScreenText(t, screen, "the first line")
	if strings.Contains(frame, "(END)") {
		t.Fatalf("the end was announced with the input still open:\n%s", frame)
	}

	// And when the input ends, the end is announced
	fmt.Fprintln(writer, "the last line")
	writer.Close()
	waitForScreenText(t, screen, "(END)")
	waitForScreenText(t, screen, "the last line")
	screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	if err := <-finished; err != nil {
		t.Fatalf("less ended with %v", err)
	}
}

// Leaving with the input still open stops the reader, rather than leaving it reading the
// shell's pipe after less has gone.
func TestLess_leavingStopsTheReader(t *testing.T) {
	screen, writer, feed, finished := pagerOnPipe(t, lessOptions{})
	fmt.Fprintln(writer, "something")
	waitForScreenText(t, screen, "something")

	// When
	screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	<-finished

	// Then
	select {
	case <-feed.done:
	case <-time.After(5 * time.Second):
		t.Fatal("the reader was still reading five seconds after less left")
	}
}

// -R takes colour sequences out, as busybox's does as it reads (miscutils/less.c:550-566), so
// `ls --color | less -R` is the names. Without it they were drawn as they came.
func TestLess_rawTakesColourSequencesOut(t *testing.T) {
	for _, test := range []struct{ in, want string }{
		{"\x1b[31mred\x1b[0m", "red"},
		{"\x1b[1;32mbold green\x1b[m plain", "bold green plain"},
		{"no colour", "no colour"},
		// An escape that is not a colour ends the discarding at its first other byte, which
		// is kept, as busybox keeps it.
		{"\x1b]0;title\x07x", "]0;title\x07x"},
	} {
		if got := lessStripColour(test.in); got != test.want {
			t.Errorf("lessStripColour(%q) = %q, want %q", test.in, got, test.want)
		}
	}
}

// A line is drawn as busybox draws it: a control character as the letter for it in reverse
// video, ESC as `[`; a tab to the next stop of eight; a wide character across two columns.
func TestLess_drawsControlsTabsAndWideCharactersInTheirColumns(t *testing.T) {
	pager, screen := newTestPager(t, []string{"a\x1bb", "x\ty", "中文z"}, 40, 5)

	// When
	pager.draw()

	// Then
	if text, style, _ := screen.Get(1, 0); text != "[" || !hasReverse(style) {
		t.Errorf("ESC is drawn as %q, reverse %v; want a reversed [", text, hasReverse(style))
	}
	if text, _, _ := screen.Get(2, 0); text != "b" {
		t.Errorf("after ESC is %q, want b", text)
	}
	if text, _, _ := screen.Get(8, 1); text != "y" {
		t.Errorf("after the tab, column 8 holds %q, want y:\n%q", text, rowOf(t, screen, 1, 40))
	}
	if text, _, _ := screen.Get(4, 2); text != "z" {
		t.Errorf("after two wide characters, column 4 holds %q, want z:\n%q", text, rowOf(t, screen, 2, 40))
	}
}

// Without a terminal it is cat, and a cat that writes a line when the line comes: it wrote
// nothing until its input ended.
func TestLessWithoutATerminal_writesWhatArrivesAsItArrives(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	out := &signallingWriter{first: make(chan struct{})}
	finished := make(chan error, 1)
	go func() {
		finished <- newLessApplet().Run(context.Background(), nil, reader, out, io.Discard)
	}()

	// When
	fmt.Fprintln(writer, "early")

	// Then: written with the input still open
	select {
	case <-out.first:
	case <-time.After(5 * time.Second):
		t.Fatal("nothing was written until the input ended")
	}
	writer.Close()
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "early\n" {
		t.Fatalf("wrote %q", got)
	}
}

// signallingWriter records what is written and says when the first write came.
type signallingWriter struct {
	mu    sync.Mutex
	text  strings.Builder
	first chan struct{}
	once  sync.Once
}

func (w *signallingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.text.Write(p)
	w.once.Do(func() { close(w.first) })
	return len(p), nil
}

func (w *signallingWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.text.String()
}

func hasReverse(style tcell.Style) bool {
	_, _, attributes := style.Decompose()
	return attributes&tcell.AttrReverse != 0
}
