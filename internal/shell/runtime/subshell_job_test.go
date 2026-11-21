package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A job a subshell starts runs on once the subshell has ended, in busybox and bash alike:
// `(cmd &)` returns at once and cmd goes on, an idiom for starting something nothing will wait
// for -- atuin's bash init ends each command's history entry so. The subshell's end cancelled
// its jobs, so the job ran nothing at all.
func TestSubshell_aJobItStartedOutlivesIt(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "marker")
	var stdout bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: new(bytes.Buffer)})
	script := "( { sleep 1; echo ran > '" + filepath.ToSlash(marker) + "'; } & ); echo after\n"
	if status := rt.RunScript(context.Background(), script); status != 0 || stdout.String() != "after\n" {
		t.Fatalf("status %d, stdout %q", status, stdout.String())
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		if data, err := os.ReadFile(marker); err == nil && string(data) == "ran\n" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the job the subshell started never ran to its end")
		}
		time.Sleep(50 * time.Millisecond)
	}
}
