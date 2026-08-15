//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// The child: launched with no console of its own, it joins the one the test is
// running in and reports whether it could.
func TestAttachConsoleHelperProcess(t *testing.T) {
	if os.Getenv("NEMOSH_ATTACH_HELPER") != "1" {
		return
	}
	report := filepath.Join(os.Getenv("NEMOSH_ATTACH_REPORT"))
	pid, _ := strconv.Atoi(os.Getenv("NEMOSH_ATTACH_PID"))
	result := "joined"
	input, output, err := attachToConsoleOf(pid)
	if err != nil {
		result = "failed: " + err.Error()
	} else {
		// Writing proves the handle is usable, not merely open. It lands in the
		// test runner's console, which is the point of the exercise.
		if _, writeErr := output.WriteString("\r"); writeErr != nil {
			result = "opened but could not write: " + writeErr.Error()
		}
		input.Close()
		output.Close()
	}
	_ = os.WriteFile(report, []byte(result), 0o600)
	os.Exit(0)
}

// The mechanism `su` runs in the current window with.
//
// It is tested unelevated, which exercises everything except the integrity
// boundary: the child is created with CREATE_NO_WINDOW so it starts with no
// console at all -- the same state ShellExecuteEx leaves an elevated child in
// under SEE_MASK_NO_CONSOLE -- and then has to find its way to this one.
//
// The elevated half cannot be tested here, because no test can answer a UAC
// prompt. What is established is that a process with no console can join
// another's and write to it, which is the part that would be surprising.
func TestAttachToConsoleOf_joinsAnotherProcessConsole(t *testing.T) {
	if !hasConsole() {
		// A CI runner is a service with no console. There is nothing to join,
		// and pretending otherwise would test the failure path only.
		t.Skip("this process has no console to join")
	}
	// Given
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(t.TempDir(), "report.txt")
	child := exec.Command(executable, "-test.run=TestAttachConsoleHelperProcess")
	child.Env = append(os.Environ(),
		"NEMOSH_ATTACH_HELPER=1",
		"NEMOSH_ATTACH_PID="+strconv.Itoa(os.Getpid()),
		"NEMOSH_ATTACH_REPORT="+report,
	)
	// CREATE_NO_WINDOW: no console of its own, which is the state it has to
	// start in for the join to be the thing under test.
	child.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x08000000}

	// When
	if err := child.Run(); err != nil {
		t.Fatalf("helper: %v", err)
	}

	// Then
	written, err := os.ReadFile(report)
	if err != nil {
		t.Fatalf("the helper wrote no report: %v", err)
	}
	if strings.TrimSpace(string(written)) != "joined" {
		t.Fatalf("helper said %q, want it to have joined this console", written)
	}
}

// The option is written by su and read by the shell it launched, so what matters
// is that it comes off cleanly and takes nothing with it.
func TestStripAttachConsoleOption(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
		pid  int
	}{
		{
			name: "the shape su launches",
			args: []string{"nemosh", "--attach-console", "4242", "-i"},
			want: []string{"nemosh", "-i"}, pid: 4242,
		},
		{
			name: "in front of a command",
			args: []string{"nemosh", "--attach-console", "7", "-c", "ls"},
			want: []string{"nemosh", "-c", "ls"}, pid: 7,
		},
		{
			name: "absent",
			args: []string{"nemosh", "-c", "ls"},
			want: []string{"nemosh", "-c", "ls"}, pid: 0,
		},
		{
			// Not a pid, so not an instruction. Left in place to be reported as
			// the invalid option it is, rather than silently eaten.
			name: "a value that is not a number",
			args: []string{"nemosh", "--attach-console", "later"},
			want: []string{"nemosh", "--attach-console", "later"}, pid: 0,
		},
		{
			name: "nothing after it",
			args: []string{"nemosh", "--attach-console"},
			want: []string{"nemosh", "--attach-console"}, pid: 0,
		},
		{
			// Only the front. Anywhere else it is somebody's argument.
			name: "not an argument of -c",
			args: []string{"nemosh", "-c", "echo --attach-console 5"},
			want: []string{"nemosh", "-c", "echo --attach-console 5"}, pid: 0,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			got, pid := stripAttachConsoleOption(test.args)

			// Then
			if pid != test.pid || strings.Join(got, " ") != strings.Join(test.want, " ") {
				t.Fatalf("stripAttachConsoleOption(%q) = (%q, %d), want (%q, %d)",
					test.args, got, pid, test.want, test.pid)
			}
		})
	}
}
