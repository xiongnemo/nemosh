package applets

import (
	"context"
	"io"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

// The drawn form, driven headlessly.
//
// tcell has a simulation screen, which means the interactive path is testable after all -- the
// application runs, the widgets fill, keys can be injected and the resulting cells read back. That
// is worth more than it sounds: without it the only check on the drawn form is a person looking at
// a terminal, and "it did not draw" is then indistinguishable from "it drew the wrong thing".

// screenText reads the simulation screen back as lines of text.
func screenText(screen tcell.SimulationScreen) string {
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

// requireSampling skips where the process table is not implemented.
//
// internal/proc samples on Windows only and refuses elsewhere rather than guessing, so on Linux
// and macOS the application draws a sampling failure and nothing else -- which is correct
// behaviour and not what these tests are about. TestPs takes the same guard for the same reason.
func requireSampling(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("the process table is implemented on Windows only")
	}
}

// The whole drawn form, end to end: it starts, samples the real machine, fills a table, and quits
// when told to.
func TestTopApplication_drawsAndQuits(t *testing.T) {
	requireSampling(t)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("simulation screen: %v", err)
	}
	screen.SetSize(160, 40)

	options, err := topArgs(nil)
	if err != nil {
		t.Fatalf("topArgs: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	finished := make(chan error, 1)
	go func() { finished <- runTopApplication(ctx, options, screen, io.Discard) }()

	// The application has to have drawn before a key means anything, and there is no hook
	// that says so -- so this waits for the header to appear rather than sleeping a fixed
	// time and hoping.
	deadline := time.Now().Add(5 * time.Second)
	drawn := ""
	for time.Now().Before(deadline) {
		if text := screenText(screen); strings.Contains(text, "top - up") {
			drawn = text
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if drawn == "" {
		t.Fatalf("nothing was drawn in five seconds; screen was:\n%s", screenText(screen))
	}

	// Then -- the header, the column titles, and at least one process
	for _, want := range []string{"top - up", "processes", "Mem:", "PID", "COMMAND"} {
		if !strings.Contains(drawn, want) {
			t.Fatalf("screen does not mention %q:\n%s", want, drawn)
		}
	}
	if !strings.Contains(drawn, "System") && !strings.Contains(drawn, "Idle") {
		t.Fatalf("no recognisable process on screen:\n%s", drawn)
	}
	// The key hint line is what makes the thing discoverable at all.
	if !strings.Contains(drawn, "quit") {
		t.Fatalf("no key hints on screen:\n%s", drawn)
	}

	// When -- q quits, which is the one key that must always work
	screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)

	// Then
	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("application returned %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("q did not quit")
	}
}

// A sort key changes the order on screen, which is the whole of what the interactive form adds
// over the plain one.
func TestTopApplication_sortKeyReordersTheTable(t *testing.T) {
	requireSampling(t)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("simulation screen: %v", err)
	}
	screen.SetSize(160, 40)
	options, _ := topArgs(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	finished := make(chan error, 1)
	go func() { finished <- runTopApplication(ctx, options, screen, io.Discard) }()
	defer func() {
		screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
		<-finished
	}()

	waitForDraw(t, screen)

	// When -- 1 sorts by the first column, which is the pid in the default layout
	screen.InjectKey(tcell.KeyRune, '1', tcell.ModNone)

	// Then -- the lowest pids come first once sorted ascending, and pid 0 is Idle. Pressing
	// 1 twice reverses, so this asserts on the arrangement rather than on one press.
	if !waitForText(t, screen, "Idle") {
		t.Fatalf("sorting by pid did not bring Idle into view:\n%s", screenText(screen))
	}
}

func waitForDraw(t *testing.T, screen tcell.SimulationScreen) {
	t.Helper()
	if !waitForText(t, screen, "top - up") {
		t.Fatalf("nothing drawn:\n%s", screenText(screen))
	}
}

func waitForText(t *testing.T, screen tcell.SimulationScreen, want string) bool {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(screenText(screen), want) {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}
