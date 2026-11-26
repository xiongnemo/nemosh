//go:build windows

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The console's modes are the session's between command lines, which only a session in a real
// console can show. btop4win crashed over ssh on 2026-11-27 and left the console's input raw,
// with mouse and window events on -- 0xb8 where the session had 0x1f7 -- and every line feed
// without its return, so each prompt after it began where the last line ended. A host in a
// hidden console of its own runs this shell there, types a line that runs a breaker -- this test
// binary again, leaving the console as btop4win did -- and then `stty -g` and two echoes, and
// reports what stty read and what the screen shows. Nothing reaches the console this test runs in.

const modesRole = "NEMOSH_MODES_ROLE"

var (
	procCreateScreenBuffer   = ctrlCKernel32.NewProc("CreateConsoleScreenBuffer")
	procReadOutputCharacters = ctrlCKernel32.NewProc("ReadConsoleOutputCharacterW")
	procWriteConsoleInput    = ctrlCKernel32.NewProc("WriteConsoleInputW")
	procVkKeyScan            = windows.NewLazySystemDLL("user32.dll").NewProc("VkKeyScanW")
)

// modesReport is what the host saw: the input mode it found the console with, what stty -g said
// at the prompt after the breaker, and the screen's rows. Console is set when the host could not
// have a console to itself.
type modesReport struct {
	Console string   `json:"console,omitempty"`
	Err     string   `json:"error,omitempty"`
	Found   string   `json:"found"`
	Stty    string   `json:"stty"`
	Screen  []string `json:"screen"`
	Kept    string   `json:"kept"`
}

// TestConsoleModesHelper is the host and the breaker, by the role the environment gives it.
func TestConsoleModesHelper(t *testing.T) {
	switch os.Getenv(modesRole) {
	case "host":
		os.Exit(consoleModesHost(os.Getenv("NEMOSH_MODES_DIR"), os.Getenv("NEMOSH_MODES_SHELL")))
	case "breaker":
		os.Exit(consoleModesBreaker(os.Getenv("NEMOSH_MODES_DIR")))
	}
	t.Skip("a helper the console modes test runs")
}

// consoleModesBreaker leaves the console as btop4win left it: the input raw with mouse and
// window events, and a screen buffer of its own whose line feed does not return. And it fails.
func consoleModesBreaker(dir string) int {
	if input, err := os.OpenFile("CONIN$", os.O_RDWR, 0); err == nil {
		_ = windows.SetConsoleMode(windows.Handle(input.Fd()), windows.ENABLE_EXTENDED_FLAGS|
			windows.ENABLE_INSERT_MODE|windows.ENABLE_MOUSE_INPUT|windows.ENABLE_WINDOW_INPUT)
	}
	second, _, _ := procCreateScreenBuffer.Call(uintptr(windows.GENERIC_READ|windows.GENERIC_WRITE),
		uintptr(windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE), 0, 1, 0)
	_ = windows.SetConsoleMode(windows.Handle(second), windows.ENABLE_PROCESSED_OUTPUT|windows.ENABLE_WRAP_AT_EOL_OUTPUT|
		windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING|windows.DISABLE_NEWLINE_AUTO_RETURN)
	_ = os.WriteFile(filepath.Join(dir, "broken"), nil, 0o644)
	return 9
}

// consoleModesHost runs in the new console: it starts the shell there with the console as its
// input and output, types its lines, and writes what it saw to report.json in dir.
func consoleModesHost(dir, shellPath string) int {
	var report modesReport
	defer func() {
		written, _ := json.Marshal(report)
		_ = os.WriteFile(filepath.Join(dir, "report.json"), written, 0o644)
	}()
	if processes, err := consoleProcesses(); err != nil || len(processes) != 1 {
		report.Console = fmt.Sprintf("the host is not alone on a console of its own: %v %v", processes, err)
		return 0
	}
	input, err := os.OpenFile("CONIN$", os.O_RDWR, 0)
	if err != nil {
		report.Console = "CONIN$: " + err.Error()
		return 0
	}
	output, err := os.OpenFile("CONOUT$", os.O_RDWR, 0)
	if err != nil {
		report.Console = "CONOUT$: " + err.Error()
		return 0
	}
	var mode uint32
	_ = windows.GetConsoleMode(windows.Handle(input.Fd()), &mode)
	report.Found = strconv.FormatUint(uint64(mode), 16)
	self, err := os.Executable()
	if err != nil {
		report.Err = err.Error()
		return 0
	}
	shell := exec.Command(shellPath, "--norc", "-i")
	shell.Env = append(os.Environ(), modesRole+"=breaker", "NEMOSH_MODES_DIR="+dir, "HOME="+dir, "ENV=")
	shell.Stdin, shell.Stdout, shell.Stderr = input, output, output
	if err := shell.Start(); err != nil {
		report.Err = err.Error()
		return 0
	}
	exited := make(chan struct{})
	go func() { _ = shell.Wait(); close(exited) }()
	defer func() {
		typeIntoConsole(input, "exit\r")
		select {
		case <-exited:
		case <-time.After(10 * time.Second):
			_ = shell.Process.Kill()
			<-exited
		}
	}()
	typeIntoConsole(input, fmt.Sprintf("\"%s\" -test.run=^TestConsoleModesHelper$\r", filepath.ToSlash(self)))
	if !awaitMarker(dir, "broken", exited) {
		report.Err = "the breaker never ran"
		return 0
	}
	time.Sleep(300 * time.Millisecond)
	typeIntoConsole(input, fmt.Sprintf("stty -g >\"%s/stty\"; echo a; echo b\r", filepath.ToSlash(dir)))
	if !awaitMarker(dir, "stty", exited) {
		report.Err = "stty -g never ran"
		return 0
	}
	time.Sleep(500 * time.Millisecond)
	saved, _ := os.ReadFile(filepath.Join(dir, "stty"))
	report.Stty = strings.TrimSpace(string(saved))
	report.Screen = consoleRows(output)
	// And stty typed on a line of its own is kept to the next.
	typeIntoConsole(input, "stty -echo\r")
	time.Sleep(300 * time.Millisecond)
	typeIntoConsole(input, fmt.Sprintf("stty -g >\"%s/kept\"; stty echo\r", filepath.ToSlash(dir)))
	if !awaitMarker(dir, "kept", exited) {
		report.Err = "the second stty -g never ran"
		return 0
	}
	time.Sleep(300 * time.Millisecond)
	kept, _ := os.ReadFile(filepath.Join(dir, "kept"))
	report.Kept = strings.TrimSpace(string(kept))
	return 0
}

// typeIntoConsole writes text into the console's input as a keyboard would, a key at a time.
func typeIntoConsole(input *os.File, text string) {
	for _, character := range text {
		scan, _, _ := procVkKeyScan.Call(uintptr(character))
		typed := consoleKey{Virtual: uint16(scan & 0xff), Character: uint16(character)}
		if character == '\r' {
			typed.Virtual = 0x0d
		}
		if scan&0x100 != 0 {
			typed.Modifiers = shiftPressed
		}
		pressConsoleKey(procWriteConsoleInput, windows.Handle(input.Fd()), typed)
		time.Sleep(2 * time.Millisecond)
	}
}

// consoleRows is the screen buffer's rows down to the cursor's, without their trailing spaces.
func consoleRows(output *os.File) []string {
	var info windows.ConsoleScreenBufferInfo
	if windows.GetConsoleScreenBufferInfo(windows.Handle(output.Fd()), &info) != nil {
		return nil
	}
	var rows []string
	for row := int16(0); row <= info.CursorPosition.Y; row++ {
		characters := make([]uint16, info.Size.X)
		var read uint32
		at := uintptr(uint32(uint16(row)) << 16)
		_, _, _ = procReadOutputCharacters.Call(output.Fd(), uintptr(unsafe.Pointer(&characters[0])), uintptr(len(characters)),
			at, uintptr(unsafe.Pointer(&read)))
		rows = append(rows, strings.TrimRight(windows.UTF16ToString(characters[:read]), " "))
	}
	return rows
}

// A program that leaves the console's modes changed when it ends leaves them to the shell, as
// cmd.exe and Git for Windows' bash take them back after one: the next prompt has the input the
// session had, and a line feed that returns.
func TestSession_givesTheConsoleBackAfterALine(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the shell and runs it in a console of its own")
	}
	nemosh := buildCtrlCShell(t)
	dir := t.TempDir()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	host := exec.CommandContext(ctx, self, "-test.run=^TestConsoleModesHelper$")
	host.Env = append(os.Environ(), modesRole+"=host", "NEMOSH_MODES_DIR="+dir, "NEMOSH_MODES_SHELL="+nemosh)
	host.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE, HideWindow: true}
	if output, err := host.CombinedOutput(); err != nil {
		t.Fatalf("host: %v\n%s", err, output)
	}
	written, err := os.ReadFile(filepath.Join(dir, "report.json"))
	if err != nil {
		t.Fatalf("the host wrote no report: %v", err)
	}
	var report modesReport
	if err := json.Unmarshal(written, &report); err != nil {
		t.Fatalf("report %q: %v", written, err)
	}
	if report.Console != "" {
		t.Skipf("no console to run the shell in: %s", report.Console)
	}
	screen := strings.Join(report.Screen, "\n")
	if report.Err != "" {
		t.Fatalf("%s; the screen:\n%s", report.Err, screen)
	}
	if in, _, _ := strings.Cut(report.Stty, ":"); in != report.Found {
		t.Errorf("after a program left the console's input as b8, stty -g at the next prompt read %q; the session had %s", report.Stty, report.Found)
	}
	if index := slices.Index(report.Screen, "a"); index < 0 || index+1 >= len(report.Screen) || report.Screen[index+1] != "b" {
		t.Errorf("echo a; echo b did not come out as two lines at the left edge:\n%s", screen)
	}
	found, _ := strconv.ParseUint(report.Found, 16, 32)
	want := strconv.FormatUint(found&^(windows.ENABLE_ECHO_INPUT|windows.ENABLE_LINE_INPUT), 16)
	if in, _, _ := strings.Cut(report.Kept, ":"); in != want {
		t.Errorf("stty -echo on a line of its own, then stty -g on the next read %q, want the input %s", report.Kept, want)
	}
}
