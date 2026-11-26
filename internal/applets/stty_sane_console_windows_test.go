//go:build windows

package applets

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// What `reset` and `stty sane` leave a real console to do with a line feed, which a mode word read
// back cannot say all of. btop4win crashed over ssh on 2026-11-27, and after it every line feed
// went down without going back, so each prompt began where the last line ended; reset did not
// help. This machine's conhost keeps whether a line feed returns once for the whole console, and
// sets it from the mode last given any screen buffer: a second buffer given
// DISABLE_NEWLINE_AUTO_RETURN takes the return from the first, whose mode word still reads 0x7,
// and only setting the first's mode again gives it back. A host in a hidden console of its own
// does that, runs reset, and reports; nothing reaches the console this test runs in.

var procCreateConsoleScreenBuffer = windows.NewLazySystemDLL("kernel32.dll").NewProc("CreateConsoleScreenBuffer")

// saneReport is what the host saw. Console is set when it had none to itself.
type saneReport struct {
	Console  string `json:"console,omitempty"`
	Taken    bool   `json:"taken"`
	Column   int    `json:"column"`
	Given    uint32 `json:"given"`
	Sane     uint32 `json:"sane"`
	SaneFail string `json:"sane_fail,omitempty"`
}

const lineFeedWithoutReturn = windows.ENABLE_PROCESSED_OUTPUT | windows.ENABLE_WRAP_AT_EOL_OUTPUT |
	windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING | windows.DISABLE_NEWLINE_AUTO_RETURN

// TestSttySaneConsoleHelper is the host, run in a console of its own.
func TestSttySaneConsoleHelper(t *testing.T) {
	if os.Getenv("NEMOSH_STTY_SANE_ROLE") != "host" {
		t.Skip("a helper the reset console test runs")
	}
	var report saneReport
	defer func() {
		written, _ := json.Marshal(report)
		_ = os.WriteFile(filepath.Join(os.Getenv("NEMOSH_STTY_SANE_DIR"), "report.json"), written, 0o644)
		os.Exit(0)
	}()
	input, err := os.OpenFile("CONIN$", os.O_RDWR, 0)
	if err != nil {
		report.Console = "CONIN$: " + err.Error()
		return
	}
	output, err := os.OpenFile("CONOUT$", os.O_RDWR, 0)
	if err != nil {
		report.Console = "CONOUT$: " + err.Error()
		return
	}
	// stty asks the process's own streams which console it is.
	os.Stdin, os.Stdout = input, output
	screen := windows.Handle(output.Fd())
	second, _, err := procCreateConsoleScreenBuffer.Call(uintptr(windows.GENERIC_READ|windows.GENERIC_WRITE),
		uintptr(windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE), 0, 1, 0)
	if windows.Handle(second) == windows.InvalidHandle {
		report.Console = "a second screen buffer: " + err.Error()
		return
	}
	_ = windows.SetConsoleMode(screen, lineFeedWithoutReturn&^windows.DISABLE_NEWLINE_AUTO_RETURN)
	_ = windows.SetConsoleMode(windows.Handle(second), lineFeedWithoutReturn)
	report.Taken = lineFeedColumn(output) != 1
	if err := newResetApplet().Run(context.Background(), nil, strings.NewReader(""), output, io.Discard); err != nil {
		report.Console = "reset: " + err.Error()
		return
	}
	report.Column = lineFeedColumn(output)

	report.Given = lineFeedWithoutReturn
	_ = windows.SetConsoleMode(screen, report.Given)
	if err := newSttyApplet().Run(context.Background(), []string{"sane"}, strings.NewReader(""), io.Discard, io.Discard); err != nil {
		report.SaneFail = err.Error()
	}
	_ = windows.GetConsoleMode(screen, &report.Sane)
}

// lineFeedColumn is the column a line feed and a letter leave the cursor in: 1 where the line
// feed returned, 2 where it went straight down.
func lineFeedColumn(output *os.File) int {
	_, _ = output.WriteString("\r\na\nb")
	var info windows.ConsoleScreenBufferInfo
	_ = windows.GetConsoleScreenBufferInfo(windows.Handle(output.Fd()), &info)
	return int(info.CursorPosition.X)
}

func TestReset_givesTheLineFeedItsReturnBack(t *testing.T) {
	if testing.Short() {
		t.Skip("runs a console of its own")
	}
	dir := t.TempDir()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	host := exec.CommandContext(ctx, self, "-test.run=^TestSttySaneConsoleHelper$")
	host.Env = append(os.Environ(), "NEMOSH_STTY_SANE_ROLE=host", "NEMOSH_STTY_SANE_DIR="+dir)
	host.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE, HideWindow: true}
	if output, err := host.CombinedOutput(); err != nil {
		t.Fatalf("host: %v\n%s", err, output)
	}
	written, err := os.ReadFile(filepath.Join(dir, "report.json"))
	if err != nil {
		t.Fatalf("the host wrote no report: %v", err)
	}
	var report saneReport
	if err := json.Unmarshal(written, &report); err != nil {
		t.Fatal(err)
	}
	if report.Console != "" {
		t.Skipf("no console to reset: %s", report.Console)
	}
	if !report.Taken {
		t.Log("this console keeps a line feed's return for each buffer, so the second took nothing from the first")
	}
	if report.Column != 1 {
		t.Errorf("after reset a line feed left the cursor in column %d, not 1: it still goes down without returning", report.Column)
	}
	if report.SaneFail != "" {
		t.Fatalf("stty sane: %s", report.SaneFail)
	}
	if want := report.Given &^ windows.DISABLE_NEWLINE_AUTO_RETURN; report.Sane != want {
		t.Errorf("stty sane left the output mode %#x from %#x, want %#x", report.Sane, report.Given, want)
	}
}
