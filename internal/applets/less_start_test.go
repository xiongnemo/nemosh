package applets

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

// less +CMD runs CMD as it starts, as GNU's does: +G at the end, +g at the start, +/PATTERN at
// the first line matching it, +?PATTERN at the last, +N at line N, and +F following. A command
// that needs a line waits for it to arrive. It was taken for a FILE named +G.
func TestLess_startCommands(t *testing.T) {
	for _, test := range []struct {
		command string
		first   string
	}{
		{"+G", "line92"},
		{"+g", "line1"},
		{"+/line50", "line50"},
		{"+?line5", "line59"},
		{"+30", "line30"},
		{"+30g", "line30"},
	} {
		start, err := parseLessStart(test.command)
		if err != nil {
			t.Fatalf("%s: %v", test.command, err)
		}
		pager, screen := newTestPager(t, lessNumberedLines(100), 40, 10)
		pager.start = start
		pager.settle()
		pager.draw()
		if got := rowOf(t, screen, 0, 40); got != test.first {
			t.Errorf("less %s starts at %q, want %q", test.command, got, test.first)
		}
	}
	for _, command := range []string{"+x", "+/", "+0", "+Gq"} {
		if _, err := parseLessStart(command); err == nil {
			t.Errorf("less %s was taken", command)
		}
	}

	// A search waits for a line it matches while more may come, and says it found none only
	// once the input has ended.
	pager, _ := newTestPager(t, []string{"early"}, 40, 5)
	pager.start, _ = parseLessStart("+/late")
	pager.waiting = true
	pager.settle()
	if pager.message != "" || pager.start.kind == 0 {
		t.Fatalf("a search with its input still open gave up: %q", pager.message)
	}
	pager.lines, pager.waiting = []string{"early", "x", "late"}, false
	pager.settle()
	if pager.message != "" || pager.start.kind != 0 {
		t.Fatalf("a search for a line that came did not find it: %q", pager.message)
	}
	pager.start, _ = parseLessStart("+/never")
	pager.settle()
	if !strings.Contains(pager.message, "not found") {
		t.Errorf("a search for what never came said %q", pager.message)
	}

	// +F follows: the prompt says it waits, and ^C stops the following rather than less.
	pager, screen := newTestPager(t, lessNumberedLines(3), 40, 5)
	pager.start, _ = parseLessStart("+F")
	pager.settle()
	pager.draw()
	if got := rowOf(t, screen, 4, 40); got != "Waiting for data... (interrupt to abort)" {
		t.Errorf("following, the prompt is %q", got)
	}
	pager.handle(tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModNone))
	if pager.following || pager.quit {
		t.Errorf("^C while following: following %v, quit %v; want following stopped and less still there", pager.following, pager.quit)
	}
}

// F follows a FILE as it grows, as tail -f does, read again from where it was after its end had
// been reached; turned off, the reading ends.
func TestLess_followsAFileAsItGrows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log")
	if err := os.WriteFile(path, []byte("a\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	feed := startLessFeed(context.Background(), []lessSource{{reader: file, close: func() { file.Close() }, native: path}})
	defer feed.stop()
	waitForFeed(t, feed, func(lines []string, ended bool) bool { return ended && len(lines) == 2 }, "the file was not read to its end")

	feed.follow(true)
	appendLine(t, path, "c\n")
	waitForFeed(t, feed, func(lines []string, ended bool) bool { return !ended && len(lines) == 3 && lines[2] == "c" }, "a line added while following was not read")
	feed.follow(false)
	waitForFeed(t, feed, func(lines []string, ended bool) bool { return ended }, "the reading went on once following stopped")
}

// :n and :p move between the FILEs, each read on its own and kept where it was, as GNU's less
// does; :x is the first again. They were one stream, the FILEs one after another.
func TestLess_nextAndPreviousFile(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{"one": "first\n", "two": "second\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ctx := WithProcessView(context.Background(), lessTestView{cwd: dir})
	files, err := openLessFiles(ctx, []string{"one", "two"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	pager, screen := newTestPager(t, nil, 40, 5)
	pager.ctx, pager.files = ctx, files
	defer pager.stopFeeds()
	pager.switchTo(0)
	shows := func(want, name string) {
		t.Helper()
		for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
			pager.pull()
			pager.draw()
			if rowOf(t, screen, 0, 40) == want && strings.Contains(rowOf(t, screen, 4, 40), name) {
				return
			}
		}
		t.Fatalf("less shows %q, %q; want %q and %q", rowOf(t, screen, 0, 40), rowOf(t, screen, 4, 40), want, name)
	}
	colon := func(key rune) {
		pager.handle(tcell.NewEventKey(tcell.KeyRune, ':', tcell.ModNone))
		pager.handle(tcell.NewEventKey(tcell.KeyRune, key, tcell.ModNone))
	}
	shows("first", "one (file 1 of 2)")
	colon('n')
	shows("second", "two (file 2 of 2)")
	colon('n')
	if pager.message != "No next file" {
		t.Errorf(":n at the last file said %q", pager.message)
	}
	colon('p')
	shows("first", "one (file 1 of 2)")
	colon('p')
	if pager.message != "No previous file" {
		t.Errorf(":p at the first file said %q", pager.message)
	}
	colon('n')
	colon('x')
	shows("first", "one (file 1 of 2)")
}

func waitForFeed(t *testing.T, feed *lessFeed, done func([]string, bool) bool, message string) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if lines, ended, _ := feed.take(); done(lines, ended) {
			return
		}
	}
	lines, ended, err := feed.take()
	t.Fatalf("%s: %q, ended %v, %v", message, lines, ended, err)
}

func appendLine(t *testing.T, path, text string) {
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

// lessTestView is a working directory for openLessFiles.
type lessTestView struct{ cwd string }

func (v lessTestView) WorkingDirectory() string        { return v.cwd }
func (v lessTestView) Environ() []string               { return nil }
func (v lessTestView) LookupEnv(string) (string, bool) { return "", false }
func (v lessTestView) ResolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(v.cwd, path)
}
