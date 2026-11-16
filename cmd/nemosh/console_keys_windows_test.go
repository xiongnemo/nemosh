//go:build windows

package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/term"
)

// What the keys this editor binds arrive as from a real console, in the virtual-terminal input
// mode raw mode sets, which a test that writes the bytes itself cannot say: AGENTS.md's ^_ was
// bound, pressed in a test, and did nothing on a keyboard. A host in a new, hidden console writes
// key records into its own input, as a keyboard would, reads them back as the editor does, and
// reports the bytes; nothing reaches the console this test runs in.

// consoleKey is one key as a keyboard puts it in a console's input: its virtual key, the
// character, and the modifiers held.
type consoleKey struct {
	Name      string
	Virtual   uint16
	Character uint16
	Modifiers uint32
}

// keyEventRecord is an INPUT_RECORD holding a KEY_EVENT_RECORD, in its documented layout.
type keyEventRecord struct {
	EventType       uint16
	_               uint16
	KeyDown         int32
	RepeatCount     uint16
	VirtualKeyCode  uint16
	VirtualScanCode uint16
	UnicodeChar     uint16
	ControlKeyState uint32
}

const (
	leftAltPressed  = 0x2
	leftCtrlPressed = 0x8
	shiftPressed    = 0x10
)

var consoleKeys = []consoleKey{
	{Name: "Backspace", Virtual: 0x08, Character: 0x08},
	{Name: "Ctrl+Backspace", Virtual: 0x08, Character: 0x7f, Modifiers: leftCtrlPressed},
	{Name: "Shift+Left", Virtual: 0x25, Modifiers: shiftPressed},
	{Name: "Ctrl+Left", Virtual: 0x25, Modifiers: leftCtrlPressed},
	{Name: "Alt+Left", Virtual: 0x25, Modifiers: leftAltPressed},
	{Name: "Ctrl+Delete", Virtual: 0x2e, Modifiers: leftCtrlPressed},
	{Name: "Delete", Virtual: 0x2e},
}

// TestConsoleKeysHelper is the host, run in its own console.
func TestConsoleKeysHelper(t *testing.T) {
	if os.Getenv("NEMOSH_KEYS_ROLE") != "host" {
		t.Skip("a helper the console keys test runs")
	}
	report := map[string]string{}
	defer func() {
		written, _ := json.Marshal(report)
		_ = os.WriteFile(filepath.Join(os.Getenv("NEMOSH_KEYS_DIR"), "report.json"), written, 0o644)
		os.Exit(0)
	}()
	if processes, err := consoleProcesses(); err != nil || len(processes) != 1 {
		report["console"] = "the host is not alone on a console of its own"
		return
	}
	name, _ := windows.UTF16PtrFromString("CONIN$")
	input, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		report["console"] = "CONIN$: " + err.Error()
		return
	}
	defer windows.CloseHandle(input)
	state, err := term.MakeRaw(int(input))
	if err != nil {
		report["console"] = "raw mode: " + err.Error()
		return
	}
	defer term.Restore(int(input), state)
	write := ctrlCKernel32.NewProc("WriteConsoleInputW")
	for _, typed := range consoleKeys {
		pressConsoleKey(write, input, typed)
		pressConsoleKey(write, input, consoleKey{Virtual: 0x5a, Character: 'z'})
		report[typed.Name] = readUntilZ(input)
	}
}

// pressConsoleKey writes a key's press and release into the console's input.
func pressConsoleKey(write *windows.LazyProc, input windows.Handle, typed consoleKey) {
	records := [2]keyEventRecord{}
	for index, down := range []int32{1, 0} {
		records[index] = keyEventRecord{EventType: 1, KeyDown: down, RepeatCount: 1, VirtualKeyCode: typed.Virtual,
			UnicodeChar: typed.Character, ControlKeyState: typed.Modifiers}
	}
	var written uint32
	_, _, _ = write.Call(uintptr(input), uintptr(unsafe.Pointer(&records[0])), 2, uintptr(unsafe.Pointer(&written)))
}

// readUntilZ is what the console hands a reader before the z that follows each key.
func readUntilZ(input windows.Handle) string {
	got := make(chan string, 1)
	go func() {
		var collected []byte
		buffer := make([]byte, 64)
		for {
			var count uint32
			if windows.ReadFile(input, buffer, &count, nil) != nil {
				got <- string(collected)
				return
			}
			for _, b := range buffer[:count] {
				if b == 'z' {
					got <- string(collected)
					return
				}
				collected = append(collected, b)
			}
		}
	}()
	select {
	case text := <-got:
		return text
	case <-time.After(3 * time.Second):
		return "(nothing)"
	}
}

// Each key the editor binds arrives from a real console as the decoder reads it.
func TestConsoleKeys_arriveAsTheEditorReadsThem(t *testing.T) {
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
	host := exec.CommandContext(ctx, self, "-test.run=^TestConsoleKeysHelper$")
	host.Env = append(os.Environ(), "NEMOSH_KEYS_ROLE=host", "NEMOSH_KEYS_DIR="+dir)
	host.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE, HideWindow: true}
	if output, err := host.CombinedOutput(); err != nil {
		t.Fatalf("host: %v\n%s", err, output)
	}
	written, err := os.ReadFile(filepath.Join(dir, "report.json"))
	if err != nil {
		t.Fatalf("the host wrote no report: %v", err)
	}
	var report map[string]string
	if err := json.Unmarshal(written, &report); err != nil {
		t.Fatal(err)
	}
	if reason, ok := report["console"]; ok {
		t.Skipf("no console to press keys in: %s", reason)
	}
	for name, want := range map[string]keyKind{
		"Backspace": keyBackspace, "Ctrl+Backspace": keyCtrlH, "Shift+Left": keyLeft, "Ctrl+Left": keyWordLeft,
		"Alt+Left": keyWordLeft, "Ctrl+Delete": keyDeleteWordForward, "Delete": keyDelete,
	} {
		bytes := report[name]
		t.Logf("%-15s %q", name, bytes)
		decoded, consumed := decodeKey([]byte(bytes))
		if decoded.kind != want || consumed != len(bytes) {
			t.Errorf("%s arrived as %q, read as key %v of %d bytes; want key %v of all of it", name, bytes, decoded.kind, consumed, want)
		}
	}
}
