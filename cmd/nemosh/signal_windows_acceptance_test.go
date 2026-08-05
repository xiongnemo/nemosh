//go:build windows

package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

const windowsSIGBREAK = syscall.Signal(21)

func TestWindowsCtrlBreakReachesProductionOsInterruptChannel(t *testing.T) {
	if os.Getenv("NEMOSH_CTRL_BREAK_HELPER") == "1" {
		interrupts, stopInterrupts := notifyInterrupts()
		breaks := make(chan os.Signal, 1)
		signal.Notify(breaks, windowsSIGBREAK)
		defer stopInterrupts()
		defer signal.Stop(breaks)
		fmt.Println("READY")
		select {
		case <-interrupts:
			fmt.Println("INTERRUPT")
		case <-breaks:
			fmt.Println("SIGBREAK")
		case <-time.After(2 * time.Second):
			fmt.Println("TIMEOUT")
		}
		os.Exit(0)
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestWindowsCtrlBreakReachesProductionOsInterruptChannel")
	cmd.Env = append(os.Environ(), "NEMOSH_CTRL_BREAK_HELPER=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	})
	lines := make(chan string, 2)
	scanErr := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		scanErr <- scanner.Err()
	}()
	if line := awaitWindowsLine(t, lines, scanErr); line != "READY" {
		t.Fatalf("helper readiness = %q", line)
	}
	if err := windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(cmd.Process.Pid)); err != nil {
		t.Fatalf("GenerateConsoleCtrlEvent(%s): %v", strconv.Itoa(cmd.Process.Pid), err)
	}
	if line := awaitWindowsLine(t, lines, scanErr); line != "INTERRUPT" {
		t.Fatalf("observed event = %q, want os.Interrupt", line)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("helper wait: %v", err)
	}
}

func TestWindowsIdleCtrlBreakRePromptsInteractiveShell(t *testing.T) {
	if os.Getenv("NEMOSH_IDLE_CTRL_BREAK_HELPER") == "1" {
		interrupts, stopInterrupts := notifyInterrupts()
		defer stopInterrupts()
		cmd := command{stdin: os.Stdin, stdout: os.Stdout, stderr: os.Stderr, interrupts: interrupts}
		if err := cmd.run(context.Background(), []string{"nemosh", "-i"}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(3)
		}
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestWindowsIdleCtrlBreakRePromptsInteractiveShell$")
	cmd.Env = append(os.Environ(), "NEMOSH_IDLE_CTRL_BREAK_HELPER=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = stdin.Close()
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	})

	stderrLines, stderrScanErr := scanWindowsLines(stderr)
	stdoutLines, stdoutScanErr := scanWindowsLines(stdout)
	if _, err := io.WriteString(stdin, "PS1='P> \\n'\n"); err != nil {
		t.Fatalf("write PS1: %v", err)
	}
	awaitWindowsLineContaining(t, stderrLines, stderrScanErr, "P> ")
	if err := windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(cmd.Process.Pid)); err != nil {
		t.Fatalf("GenerateConsoleCtrlEvent(%s): %v", strconv.Itoa(cmd.Process.Pid), err)
	}
	awaitWindowsLineContaining(t, stderrLines, stderrScanErr, "P> ")
	if _, err := io.WriteString(stdin, "echo alive\nexit 0\n"); err != nil {
		t.Fatalf("write recovery commands: %v", err)
	}
	if line := awaitWindowsLine(t, stdoutLines, stdoutScanErr); line != "alive" {
		t.Fatalf("recovery output = %q, want alive", line)
	}
	if err := stdin.Close(); err != nil {
		t.Fatalf("close stdin: %v", err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("helper wait: %v", err)
	}
}

func scanWindowsLines(reader io.Reader) (<-chan string, <-chan error) {
	lines := make(chan string, 4)
	scanErr := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
		scanErr <- scanner.Err()
	}()
	return lines, scanErr
}

func awaitWindowsLineContaining(t *testing.T, lines <-chan string, scanErr <-chan error, text string) {
	t.Helper()
	for range 4 {
		if strings.Contains(awaitWindowsLine(t, lines, scanErr), text) {
			return
		}
	}
	t.Fatalf("did not observe line containing %q", text)
}

func awaitWindowsLine(t *testing.T, lines <-chan string, scanErr <-chan error) string {
	t.Helper()
	select {
	case line := <-lines:
		return line
	case err := <-scanErr:
		t.Fatalf("helper output ended: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("helper output timed out")
	}
	return ""
}
