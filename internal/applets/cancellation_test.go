package applets_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// Cancellation, as a property of the whole applet set rather than of any one
// applet.
//
// Thirteen applets check `ctx.Done()` explicitly and the rest inherit the check
// their runner makes. Either way the contract is the same and is what makes Ctrl-C
// work: **a cancelled context means the applet stops, reports it, and does not
// write anything it had not already written.** That is one property with many
// implementations, so it gets one test that walks them.
//
// This is also the branch coverage kept finding uncovered file by file. Testing it
// per applet would be thirteen near-identical tests; testing it as a property is
// one, and it covers the applets that inherit the check as well.

// Every applet that takes no operand, or takes one this test can supply, run with a
// context that is already cancelled.
func TestApplets_stopOnACancelledContext(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("one\ntwo\nthree\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name string
		args []string
	}{
		// The ones with an explicit check, which are the ones that would loop
		// forever or sample the machine if they did not have it.
		{name: "yes"},
		{name: "sleep", args: []string{"60"}},
		{name: "cat", args: []string{"a.txt"}},
		{name: "sort", args: []string{"a.txt"}},
		{name: "uniq", args: []string{"a.txt"}},
		{name: "cut", args: []string{"-c", "1", "a.txt"}},
		{name: "date"},
		{name: "uname", args: []string{"-a"}},
		{name: "ps"},
		{name: "top", args: []string{"-b", "-n", "1"}},
		{name: "winpath", args: []string{"/c/x"}},
		{name: "posixpath", args: []string{"C:/x"}},
		// And a sample of the ones that inherit it, to show the contract is the
		// set's and not a habit of a few files.
		{name: "wc", args: []string{"a.txt"}},
		{name: "head", args: []string{"a.txt"}},
		{name: "grep", args: []string{"one", "a.txt"}},
		{name: "ls"},
	} {
		t.Run(test.name, func(t *testing.T) {
			applet, ok := applets.DefaultRegistry.Lookup(test.name)
			if !ok {
				t.Skipf("%s is not registered on this platform", test.name)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			ctx = applets.WithProcessView(ctx, findTestProcessView{cwd: root})

			var stdout, stderr strings.Builder
			err := applet.Run(ctx, test.args, strings.NewReader("one\ntwo\n"), &stdout, &stderr)

			// Stopping is the requirement; *how* it reports is not uniform, because
			// some applets answer context.Canceled and others fail on the first read
			// they attempt. What must not happen is a report of success.
			if err == nil {
				t.Fatalf("%s reported success on a cancelled context (stdout %q)", test.name, stdout.String())
			}
		})
	}
}

// The sharper half of the same contract: an applet that would otherwise never stop
// must actually return, rather than returning an error from a goroutine while the
// loop runs on.
//
// `yes` is the case -- it writes forever by definition -- and this asserts it comes
// back at all. Without the check the test would hang and be killed by the package
// timeout, which is a failure but a slow and confusing one, so the bound is stated.
func TestYes_returnsWhenCancelledRatherThanRunningOn(t *testing.T) {
	applet, ok := applets.DefaultRegistry.Lookup("yes")
	if !ok {
		t.Skip("yes is not registered")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	finished := make(chan error, 1)
	go func() {
		// io.Discard, not a buffer: an unbounded writer is the point, and a buffer
		// would turn a missing check into an out-of-memory rather than a timeout.
		finished <- applet.Run(ctx, nil, strings.NewReader(""), discardWriter{}, discardWriter{})
	}()
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("yes reported success on a cancelled context")
		}
	case <-timeoutAfter():
		t.Fatal("yes did not return on a cancelled context")
	}
}

// A context cancelled *part-way* through, rather than before the applet starts. The
// two are different branches: the first is checked before any work, the second in
// the loop, and only the second is what Ctrl-C actually exercises.
func TestSleep_stopsPartWayThrough(t *testing.T) {
	applet, ok := applets.DefaultRegistry.Lookup("sleep")
	if !ok {
		t.Skip("sleep is not registered")
	}
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan error, 1)
	go func() {
		finished <- applet.Run(ctx, []string{"300"}, strings.NewReader(""), discardWriter{}, discardWriter{})
	}()
	// Cancelled after the applet is under way rather than before it begins.
	cancel()
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("sleep slept through its cancellation and reported success")
		}
	case <-timeoutAfter():
		t.Fatal("sleep did not wake on cancellation, so a five-minute sleep is uninterruptible")
	}
}

// timeoutAfter bounds the two tests that would otherwise hang. Ten seconds is long
// enough that a slow CI runner is not mistaken for a missing check, and short enough
// that the failure names itself rather than arriving as a package timeout.
func timeoutAfter() <-chan time.Time { return time.After(10 * time.Second) }

type discardWriter struct{}

func (discardWriter) Write(data []byte) (int, error) { return len(data), nil }
