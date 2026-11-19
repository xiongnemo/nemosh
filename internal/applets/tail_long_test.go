package applets_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// head and tail take GNU's long options, which busybox's have not: --lines, --bytes, --quiet,
// --silent and --verbose, and tail's --follow, --retry, --sleep-interval and --pid. Each was
// `unknown option`.
func TestHeadTail_takeGnusLongOptions(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{"a": "1\n2\n3\n4\n5\n", "b": "x\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ctx := applets.WithProcessView(context.Background(), permuteTestView{cwd: dir})
	run := func(args ...string) (string, string, error) {
		applet, _ := applets.DefaultRegistry.Lookup(args[0])
		var stdout, stderr bytes.Buffer
		err := applet.Run(ctx, args[1:], strings.NewReader(""), &stdout, &stderr)
		return stdout.String(), stderr.String(), err
	}
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"tail", "--lines=2", "a"}, "4\n5\n"},
		{[]string{"tail", "--lines", "2", "a"}, "4\n5\n"},
		{[]string{"tail", "--lines=+4", "a"}, "4\n5\n"},
		{[]string{"tail", "--bytes=2", "a"}, "5\n"},
		{[]string{"head", "--lines=2", "a"}, "1\n2\n"},
		{[]string{"head", "--bytes=3", "a"}, "1\n2"},
		{[]string{"head", "--quiet", "--lines=1", "a", "b"}, "1\nx\n"},
		{[]string{"tail", "--silent", "--lines=1", "a", "b"}, "5\nx\n"},
		{[]string{"tail", "--verbose", "--lines=1", "b"}, "==> b <==\nx\n"},
	} {
		if stdout, stderr, err := run(test.args...); err != nil || stdout != test.want {
			t.Errorf("%q = %q, %v (%s); want %q", test.args, stdout, err, stderr, test.want)
		}
	}
	for _, args := range [][]string{
		{"tail", "--follow=sideways", "a"},
		{"head", "--quiet=yes", "a"},
		{"head", "--follow", "a"},
		{"tail", "--pid=x", "a"},
		{"tail", "--pid=0", "a"},
		{"tail", "--lines"},
		{"tail", "--sleep-interval=x", "a"},
	} {
		if _, _, err := run(args...); err == nil {
			t.Errorf("%q succeeded", args)
		}
	}
	// Without -f there is nothing to stop, and GNU's tail says so and prints.
	stdout, stderr, err := run("tail", "--pid="+strconv.Itoa(os.Getpid()), "--lines=1", "a")
	if err != nil || stdout != "5\n" || stderr != "tail: warning: PID ignored; --pid=PID is useful only when following\n" {
		t.Errorf("tail --pid without -f = %q, %q, %v", stdout, stderr, err)
	}
}

// tail -f --pid=PID follows until the process PID ends, then reads what it wrote and stops with
// status 0, as GNU's does: `tail -f --pid=$! log`. One above this process's pid is no process,
// for a Windows pid is a multiple of four, though OpenProcess would open this process for it.
func TestTail_pidEndsTheFollow(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "log")
	if err := os.WriteFile(log, []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := applets.WithProcessView(context.Background(), permuteTestView{cwd: dir})
	follow := func(pid int) (*lockedBuffer, chan error) {
		applet, _ := applets.DefaultRegistry.Lookup("tail")
		stdout, done := &lockedBuffer{}, make(chan error, 1)
		go func() {
			done <- applet.Run(ctx, []string{"-f", "-s", "1", "--pid=" + strconv.Itoa(pid), "log"}, strings.NewReader(""), stdout, &lockedBuffer{})
		}()
		return stdout, done
	}
	stopsBy := func(done chan error, what string) {
		t.Helper()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("tail -f --pid, %s: %v", what, err)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("tail -f --pid, %s, never stopped", what)
		}
	}

	stdout, done := follow(os.Getpid() + 1)
	stopsBy(done, "no such process")
	if stdout.String() != "1\n" {
		t.Errorf("tail -f --pid of no process printed %q, want %q", stdout.String(), "1\n")
	}

	child := exec.Command(os.Args[0], "-test.run=^TestTailPidHelper$")
	child.Env = append(os.Environ(), "NEMOSH_TAIL_PID_HELPER=1")
	if err := child.Start(); err != nil {
		t.Fatalf("starting the helper: %v", err)
	}
	defer child.Process.Kill()
	stdout, done = follow(child.Process.Pid)
	if !comesToHold(stdout, "1\n") {
		t.Fatalf("tail -f --pid printed %q", stdout.String())
	}
	select {
	case err := <-done:
		t.Fatalf("tail -f --pid stopped while the process ran: %v", err)
	case <-time.After(1500 * time.Millisecond):
	}
	appendTo(t, log, "2\n")
	child.Process.Kill()
	child.Wait()
	stopsBy(done, "once the process ended")
	if stdout.String() != "1\n2\n" {
		t.Errorf("tail -f --pid printed %q, want %q", stdout.String(), "1\n2\n")
	}
}

// TestTailPidHelper is the process TestTail_pidEndsTheFollow watches: it waits to be stopped.
func TestTailPidHelper(t *testing.T) {
	if os.Getenv("NEMOSH_TAIL_PID_HELPER") == "" {
		t.Skip("the child of TestTail_pidEndsTheFollow")
	}
	time.Sleep(time.Minute)
}

func appendTo(t *testing.T, path, text string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.WriteString(text); err != nil {
		t.Fatal(err)
	}
}
