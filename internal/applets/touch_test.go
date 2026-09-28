package applets_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// touch sets an existing file's times to now, leaves a missing one alone under -c, and goes on
// past an operand it cannot touch, as busybox's does: it names that one on stderr, touches the
// rest, and exits 1. It only created files. An existing one kept its old time, -c created what it
// named, and the first failure abandoned the operands after it.
func TestTouch_setsTimesSkipsUnderDashCAndGoesOnPastAFailure(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "old.txt")
	if err := os.WriteFile(old, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}
	touch, ok := applets.DefaultRegistry.Lookup("touch")
	if !ok {
		t.Fatal("touch is not registered")
	}
	ctx := applets.WithProcessView(context.Background(), diagnosticTestView{cwd: dir})
	run := func(args ...string) (string, error) {
		var stderr bytes.Buffer
		err := touch.Run(ctx, args, &bytes.Buffer{}, &bytes.Buffer{}, &stderr)
		return stderr.String(), err
	}

	if _, err := run("old.txt"); err != nil {
		t.Fatalf("touch old.txt: %v", err)
	}
	if info, err := os.Stat(old); err != nil || time.Since(info.ModTime()) > time.Hour {
		t.Fatalf("old.txt's time is %v (err %v), want now", info.ModTime(), err)
	}

	if _, err := run("-c", "absent.txt"); err != nil {
		t.Fatalf("touch -c absent.txt: %v, want status 0", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "absent.txt")); !os.IsNotExist(err) {
		t.Fatalf("touch -c created absent.txt (stat err %v)", err)
	}

	stderr, err := run("nodir/a.txt", "made.txt")
	if code, isStatus := applets.StatusCode(err); !isStatus || code != 1 {
		t.Fatalf("touch nodir/a.txt made.txt returned %v, want status 1", err)
	}
	if !strings.Contains(stderr, "nodir/a.txt: No such file or directory") {
		t.Fatalf("stderr = %q, want the failing operand named", stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "made.txt")); err != nil {
		t.Fatalf("made.txt after the failure: %v, want it touched", err)
	}
}
