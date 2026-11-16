//go:build windows

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Ctrl-C at a console reaches every process attached to it: the shell, and the program it is
// running in the foreground, which is how that program gets to handle it -- ping prints its
// statistics, a nested cmd or pwsh stops the command it is running and stays, python unwinds.
// What the shell then does with the program is what these tests measure, against busybox-w32,
// in a console of their own with nothing else on it.
//
// A console is needed for that, and broadcasting into this test's own would interrupt go test
// itself, so the test starts a host in a new, hidden console. The host runs the shell there
// with a probe -- this test binary again -- as its foreground program, waits for the probe to
// say it is ready, presses Ctrl-C (GenerateConsoleCtrlEvent to group 0, everything on the
// console), and reports what the shell and the probe did. Every role ends by itself within
// seconds, so nothing outlives a run.
//
// Measured 2026-11-16 with busybox-w32 FRP-6354: busybox ends the program, in a script and at
// `-ic` alike -- one that handles Ctrl-C and one that ignores it -- unless it is the whole of
// `-ic`'s command line, which busybox hands the process over to. nemosh did the same. Ended, a
// nested cmd, pwsh or python at the prompt was closed with whatever it was running.

const ctrlCRole = "NEMOSH_CTRLC_ROLE"

var (
	ctrlCKernel32             = windows.NewLazySystemDLL("kernel32.dll")
	procSetConsoleCtrlHandler = ctrlCKernel32.NewProc("SetConsoleCtrlHandler")
	procGetConsoleProcessList = ctrlCKernel32.NewProc("GetConsoleProcessList")
)

// ctrlCReport is what the host saw: the shell's status, how long after the event it exited, and
// which of the probe's markers were there then and a while later. Console is set when the host
// could not have a console to itself, which a machine may not give it.
type ctrlCReport struct {
	Status  int      `json:"status"`
	Elapsed int64    `json:"elapsed_ms"`
	AtExit  []string `json:"at_exit"`
	Later   []string `json:"later"`
	Console string   `json:"console,omitempty"`
	Err     string   `json:"error,omitempty"`
}

func (r ctrlCReport) String() string {
	if r.Err != "" || r.Console != "" {
		return "error: " + r.Err + r.Console
	}
	return fmt.Sprintf("status %d after %d ms; at exit %v; later %v", r.Status, r.Elapsed, r.AtExit, r.Later)
}

// TestCtrlCConsoleHelper is the host and the probe, by the role the environment gives it.
func TestCtrlCConsoleHelper(t *testing.T) {
	switch os.Getenv(ctrlCRole) {
	case "host":
		os.Exit(ctrlCHost(os.Getenv("NEMOSH_CTRLC_DIR")))
	case "probe":
		os.Exit(ctrlCProbe(os.Getenv("NEMOSH_CTRLC_PROBE"), os.Getenv("NEMOSH_CTRLC_DIR")))
	}
	t.Skip("a helper the Ctrl-C tests run")
}

// ctrlCProbe is the foreground program: one that handles Ctrl-C and takes 300 ms to finish
// doing so, one that leaves it to Windows' default, which ends it with STATUS_CONTROL_C_EXIT,
// and one that ignores it and goes on.
func ctrlCProbe(kind, dir string) int {
	mark := func(name string) { _ = os.WriteFile(filepath.Join(dir, name), nil, 0o644) }
	switch kind {
	case "handle":
		interrupts := make(chan os.Signal, 1)
		signal.Notify(interrupts, os.Interrupt)
		mark("ready")
		select {
		case <-interrupts:
			time.Sleep(300 * time.Millisecond)
			mark("handled")
			return 42
		case <-time.After(8 * time.Second):
			mark("timeout")
			return 3
		}
	case "survive":
		_, _, _ = procSetConsoleCtrlHandler.Call(0, 1)
		mark("ready")
		time.Sleep(1500 * time.Millisecond)
		mark("survived")
		return 0
	default:
		mark("ready")
		time.Sleep(8 * time.Second)
		mark("timeout")
		return 3
	}
}

// ctrlCHost runs in the new console: it starts the shell there, presses Ctrl-C once the probe is
// ready, and writes what happened to report.json in dir.
func ctrlCHost(dir string) int {
	report := ctrlCReport{Status: -1}
	defer func() {
		written, _ := json.Marshal(report)
		_ = os.WriteFile(filepath.Join(dir, "report.json"), written, 0o644)
	}()
	// Processing Ctrl-C is a flag a process inherits; a parent started in a group of its own,
	// as a CI runner's step is, passes on one that ignores it.
	_, _, _ = procSetConsoleCtrlHandler.Call(0, 0)
	interrupts := make(chan os.Signal, 4)
	signal.Notify(interrupts, os.Interrupt)
	if processes, err := consoleProcesses(); err != nil || len(processes) != 1 {
		report.Console = fmt.Sprintf("the host is not alone on a console of its own: %v %v", processes, err)
		return 0
	}
	var words []string
	if err := json.Unmarshal([]byte(os.Getenv("NEMOSH_CTRLC_ARGS")), &words); err != nil || len(words) == 0 {
		report.Err = fmt.Sprintf("arguments: %v", err)
		return 0
	}
	shell := exec.Command(words[0], words[1:]...)
	shell.Env = append(os.Environ(), ctrlCRole+"=probe", "HOME="+dir, "ENV=")
	if input := os.Getenv("NEMOSH_CTRLC_STDIN"); input != "" {
		file, err := os.Open(input)
		if err != nil {
			report.Err = err.Error()
			return 0
		}
		defer file.Close()
		shell.Stdin = file
	}
	output, err := os.Create(filepath.Join(dir, "shell.out"))
	if err != nil {
		report.Err = err.Error()
		return 0
	}
	defer output.Close()
	shell.Stdout, shell.Stderr = output, output
	if err := shell.Start(); err != nil {
		report.Err = err.Error()
		return 0
	}
	exited := make(chan struct{})
	go func() { _ = shell.Wait(); close(exited) }()
	if !awaitMarker(dir, "ready", exited) {
		report.Err = "the probe never said it was ready"
		_ = shell.Process.Kill()
		<-exited
		return 0
	}
	event := uint32(windows.CTRL_C_EVENT)
	if os.Getenv("NEMOSH_CTRLC_EVENT") == "break" {
		event = windows.CTRL_BREAK_EVENT
	}
	sent := time.Now()
	if err := windows.GenerateConsoleCtrlEvent(event, 0); err != nil {
		report.Console = "GenerateConsoleCtrlEvent: " + err.Error()
	}
	select {
	case <-exited:
	case <-time.After(10 * time.Second):
		report.Err = "the shell was still running 10 s after the event"
		_ = shell.Process.Kill()
		<-exited
	}
	report.Elapsed = time.Since(sent).Milliseconds()
	report.Status = shell.ProcessState.ExitCode()
	report.AtExit = ctrlCMarkers(dir)
	time.Sleep(1500 * time.Millisecond)
	report.Later = ctrlCMarkers(dir)
	return 0
}

func consoleProcesses() ([]uint32, error) {
	list := make([]uint32, 64)
	count, _, err := procGetConsoleProcessList.Call(uintptr(unsafe.Pointer(&list[0])), uintptr(len(list)))
	if count == 0 {
		return nil, err
	}
	return list[:count], nil
}

func awaitMarker(dir, name string, exited <-chan struct{}) bool {
	deadline := time.After(10 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
		select {
		case <-exited:
			return false
		case <-deadline:
			return false
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// ctrlCMarkers are the probe's markers there are, and what the script wrote after it.
func ctrlCMarkers(dir string) []string {
	var found []string
	for _, name := range []string{"handled", "survived", "timeout", "after"} {
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		if text := strings.TrimSpace(string(content)); text != "" {
			name += "=" + text
		}
		found = append(found, name)
	}
	return found
}

// ctrlCRun is one press of Ctrl-C: the shell's command line before its script, the script,
// typed at the shell's prompt or given as the operand of the shell's last word, the probe the
// script runs as P, and the environment the shell is given over the test's.
type ctrlCRun struct {
	shell  []string
	typed  bool
	script string
	probe  string
	event  string
	env    []string
}

// runCtrlC runs it in a console of its own and reports what happened. In the script P is the
// probe and D its directory.
func runCtrlC(t *testing.T, run ctrlCRun) ctrlCReport {
	t.Helper()
	dir := t.TempDir()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	script := fmt.Sprintf("P='%s'; D='%s'; %s", filepath.ToSlash(self), filepath.ToSlash(dir),
		strings.ReplaceAll(run.script, "P", `"$P" -test.run=^TestCtrlCConsoleHelper$`))
	words := slices.Clone(run.shell)
	stdin := ""
	if run.typed {
		stdin = filepath.Join(dir, "typed")
		if err := os.WriteFile(stdin, []byte(script+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	} else {
		words = append(words, script)
	}
	encoded, _ := json.Marshal(words)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	host := exec.CommandContext(ctx, self, "-test.run=^TestCtrlCConsoleHelper$")
	host.Env = append(os.Environ(), ctrlCRole+"=host", "NEMOSH_CTRLC_DIR="+dir, "NEMOSH_CTRLC_ARGS="+string(encoded),
		"NEMOSH_CTRLC_STDIN="+stdin, "NEMOSH_CTRLC_PROBE="+run.probe, "NEMOSH_CTRLC_EVENT="+run.event)
	host.Env = append(host.Env, run.env...)
	host.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE, HideWindow: true}
	if output, err := host.CombinedOutput(); err != nil {
		t.Fatalf("host: %v\n%s", err, output)
	}
	written, err := os.ReadFile(filepath.Join(dir, "report.json"))
	if err != nil {
		t.Fatalf("the host wrote no report: %v", err)
	}
	var report ctrlCReport
	if err := json.Unmarshal(written, &report); err != nil {
		t.Fatalf("report %q: %v", written, err)
	}
	if report.Console != "" {
		t.Skipf("no console to press Ctrl-C in: %s", report.Console)
	}
	if report.Err != "" {
		t.Fatalf("%s", report.Err)
	}
	if shellOutput, err := os.ReadFile(filepath.Join(dir, "shell.out")); err == nil && len(shellOutput) > 0 {
		t.Logf("the shell said %q", shellOutput)
	}
	return report
}

// ctrlCBusybox is the busybox to measure against, through the shim a package manager put on
// PATH: a shim is a console program of its own between the test and busybox.
func ctrlCBusybox() string {
	if path := os.Getenv("NEMOSH_CTRLC_BUSYBOX"); path != "" {
		return path
	}
	found, err := exec.LookPath("busybox")
	if err != nil {
		return ""
	}
	shim := strings.TrimSuffix(found, filepath.Ext(found)) + ".shim"
	if file, err := os.Open(shim); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			if key, value, ok := strings.Cut(scanner.Text(), "="); ok && strings.TrimSpace(key) == "path" {
				return strings.Trim(strings.TrimSpace(value), `"`)
			}
		}
	}
	if tools, _ := filepath.Glob(filepath.Join(filepath.Dir(filepath.Dir(found)), "lib", "busybox", "tools", "busybox*.exe")); len(tools) > 0 {
		return tools[0]
	}
	return found
}

// buildCtrlCShell builds this shell, to be what the host runs.
func buildCtrlCShell(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "ctrlcshell.exe")
	build := exec.Command("go", "build", "-o", binary, "github.com/xiongnemo/nemosh/cmd/nemosh")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the shell: %v\n%s", err, output)
	}
	return binary
}

// A program run from the prompt is left to handle Ctrl-C, as cmd leaves it and bash does: it
// finishes, and the prompt is back when it has. In a script it is ended, as busybox ends it,
// unless NEMOSH_CTRL_C=program; NEMOSH_CTRL_C=kill ends it at the prompt too.
//
// Where it should be ended the probe is the one that ignores Ctrl-C and goes on for 1.5 s, so
// being ended is told from finishing by 1.5 s and not by the 300 ms the other takes.
func TestCtrlC_aProgramIsLeftToItAtThePromptAndEndedInAScript(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the shell and runs it in a console of its own")
	}
	nemosh := buildCtrlCShell(t)
	for _, test := range []struct {
		name    string
		typed   bool
		probe   string
		env     []string
		handled bool
	}{
		{name: "at the prompt it finishes handling it", typed: true, probe: "handle", handled: true},
		{name: "at the prompt one that ignores it goes on", typed: true, probe: "survive", handled: true},
		{name: "at the prompt NEMOSH_CTRL_C=kill ends it", typed: true, probe: "survive", env: []string{"NEMOSH_CTRL_C=kill"}},
		{name: "in a script it is ended", probe: "survive"},
		{name: "in a script NEMOSH_CTRL_C=program leaves it", probe: "handle", env: []string{"NEMOSH_CTRL_C=program"}, handled: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			shell := []string{nemosh, "-c"}
			if test.typed {
				shell = []string{nemosh, "-i"}
			}

			// When
			report := runCtrlC(t, ctrlCRun{shell: shell, typed: test.typed, script: "P; echo after >\"$D/after\"",
				probe: test.probe, event: "c", env: test.env})

			// Then
			t.Log(report)
			finished := slices.ContainsFunc(report.Later, func(marker string) bool { return marker == "handled" || marker == "survived" })
			if finished != test.handled {
				t.Fatalf("the program finished: %v, want %v (%s)", finished, test.handled, report)
			}
			if test.handled && !slices.Equal(report.AtExit, report.Later) {
				t.Errorf("the shell left before the program finished: %s", report)
			}
			if slices.Contains(report.Later, "after") {
				t.Errorf("the line went on after Ctrl-C: %s", report)
			}
		})
	}
}

// TestCtrlC_measureAgainstBusybox prints what each shell does, for deciding; it asserts nothing.
// NEMOSH_CTRLC_MATRIX=1 runs it, and =busybox only busybox's half.
//
// It was an assertion that busybox ends the program, and the gate's load showed why it cannot be
// one: idle, busybox ended it within 40 ms of the Ctrl-C; under the gate it took 322 ms once and
// 2522 ms another time, long enough for the probe that ignores Ctrl-C to have finished.
func TestCtrlC_measureAgainstBusybox(t *testing.T) {
	if os.Getenv("NEMOSH_CTRLC_MATRIX") == "" {
		t.Skip("NEMOSH_CTRLC_MATRIX=1 prints the measurement")
	}
	shells := map[string][]string{}
	if busybox := ctrlCBusybox(); busybox != "" {
		shells["busybox"] = []string{busybox, "sh"}
	}
	if os.Getenv("NEMOSH_CTRLC_MATRIX") != "busybox" {
		shells["nemosh"] = []string{buildCtrlCShell(t)}
	}
	scripts := []string{
		"P; echo after:$? >\"$D/after\"",
		"P | cat; echo after:$? >\"$D/after\"",
		"echo x | xargs P; echo after:$? >\"$D/after\"",
		"v=$(P); echo after:$? >\"$D/after\"",
		"P",
	}
	for _, name := range []string{"busybox", "nemosh"} {
		shell, ok := shells[name]
		if !ok {
			continue
		}
		for _, flag := range []string{"-c", "-ic"} {
			for _, script := range scripts {
				for _, probe := range []string{"handle", "default", "survive"} {
					report := runCtrlC(t, ctrlCRun{shell: append(slices.Clone(shell), flag), script: script, probe: probe, event: "c"})
					t.Logf("%-7s %-3s %-8s %-46s %s", name, flag, probe, script, report)
				}
			}
		}
		for _, probe := range []string{"handle", "default", "survive"} {
			report := runCtrlC(t, ctrlCRun{shell: append(slices.Clone(shell), "-c"), script: scripts[0], probe: probe, event: "break"})
			t.Logf("%-7s -c  %-8s %-46s %s (Ctrl-Break)", name, probe, scripts[0], report)
		}
	}
}
