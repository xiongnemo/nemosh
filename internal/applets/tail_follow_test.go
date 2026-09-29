package applets_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// tail opens every FILE before it prints any, as busybox's tail_main does: one it cannot open
// is named first, and the headers count the ones that opened, the first with no blank line
// above it. It printed as head does, a FILE at a time.
func TestTail_opensEveryFileFirst(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{"a": "x\n", "b": "y\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	applet, _ := applets.DefaultRegistry.Lookup("tail")
	ctx := applets.WithProcessView(context.Background(), permuteTestView{cwd: dir})
	missing := "tail: cannot open 'nosuch': No such file or directory\n"
	for _, test := range []struct {
		args []string
		want string
	}{
		{[]string{"nosuch", "a"}, missing + "x\n"},
		{[]string{"nosuch", "a", "b"}, missing + "==> a <==\nx\n\n==> b <==\ny\n"},
		{[]string{"a", "nosuch", "b"}, missing + "==> a <==\nx\n\n==> b <==\ny\n"},
	} {
		var both bytes.Buffer
		err := applet.Run(ctx, test.args, strings.NewReader(""), &both, &both)
		if status, _ := applets.StatusCode(err); status != 1 || both.String() != test.want {
			t.Errorf("tail %q 2>&1: %q, %v; want %q and status 1", test.args, both.String(), err, test.want)
		}
	}
}

// lockedBuffer is stdout for a tail -f that runs while the test reads what it wrote.
type lockedBuffer struct {
	mutex  sync.Mutex
	buffer bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.buffer.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.buffer.String()
}

// comesToHold is whether buffer comes to hold want within a few seconds.
func comesToHold(buffer *lockedBuffer, want string) bool {
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if strings.Contains(buffer.String(), want) {
			return true
		}
	}
	return false
}

// tail -f prints what is added to a FILE, and from its start again once it has shrunk; -F
// follows a FILE its name comes to mean, and one that appears. Each case was measured against
// busybox-w32's tail, but for the rename, which busybox-w32 cannot follow: it holds the FILE
// so that `mv` is refused, where this lets the writer rotate its log. Each ends as ^C ends it.
func TestTail_followsAsBusyboxDoes(t *testing.T) {
	dir := t.TempDir()
	path := func(name string) string { return filepath.Join(dir, name) }
	write := func(name, content string, flags int) {
		t.Helper()
		file, err := os.OpenFile(path(name), flags|os.O_WRONLY|os.O_CREATE, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		file.WriteString(content)
		file.Close()
	}
	write("log", "one\ntwo\n", os.O_TRUNC)
	applet, _ := applets.DefaultRegistry.Lookup("tail")
	ctx, cancel := context.WithCancel(applets.WithProcessView(context.Background(), permuteTestView{cwd: dir}))
	var stdout, stderr lockedBuffer
	done := make(chan error, 1)
	go func() {
		done <- applet.Run(ctx, []string{"-F", "-s", "0", "log", "late"}, strings.NewReader(""), &stdout, &stderr)
	}()
	steps := []struct {
		act            func()
		stdout, stderr string
	}{
		{func() {}, "==> log <==\none\ntwo\n", "tail: cannot open 'late': No such file or directory\n"},
		{func() { write("log", "three\n", os.O_APPEND) }, "two\nthree\n", ""},
		{func() { write("log", "x\n", os.O_TRUNC) }, "three\nx\n", ""},
		{func() {
			if err := os.Rename(path("log"), path("log.1")); err != nil {
				t.Fatalf("rotating the log tail follows: %v", err)
			}
			write("log", "fresh\n", os.O_TRUNC)
		}, "x\nfresh\n", "tail: log has been replaced; following end of new file\n"},
		{func() { write("late", "here\n", os.O_TRUNC) }, "\n==> late <==\nhere\n", "tail: late has appeared; following end of new file\n"},
	}
	for index, step := range steps {
		step.act()
		if !comesToHold(&stdout, step.stdout) || !comesToHold(&stderr, step.stderr) {
			cancel()
			t.Fatalf("step %d: stdout %q, stderr %q; want them to hold %q and %q", index, stdout.String(), stderr.String(), step.stdout, step.stderr)
		}
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("tail -F ended with no error when it was stopped")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("tail -F went on after it was stopped")
	}
}
