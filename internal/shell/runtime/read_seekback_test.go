package runtime_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// read takes a line from a file on disk a block at a time, and seeks back over what the line
// did not use, so the next command finds the file where the line ended: after `read a` and
// `read -n 2 b`, cat prints the rest, and a line longer than a block is read whole.
func TestRuntime_readLeavesAFileWhereTheLineEnded(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("x", 1500)
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte("one\ntwo,three\n"+long+"\nlast\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})
	script := "cd '" + filepath.ToSlash(dir) + "'\n" +
		"{ read a; read -d , b; read -n 2 c; read d; read e; echo \"$a|$b|$c|$d|${#e}\"; cat; } < f\n"
	status := rt.RunScript(context.Background(), script)
	rt.CloseBatch(status)
	if want := "one|two|th|ree|1500\nlast\n"; stdout.String() != want || stderr.String() != "" {
		t.Errorf("got %q, %q; want %q", stdout.String(), stderr.String(), want)
	}
}
