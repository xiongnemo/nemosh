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

// A name that is there and will not run -- a file that is no program, a directory -- is
// "Permission denied", 126, as busybox has it; the hint says which it is. A data file was
// "not found", 127, as though it were not there, and a directory said the host path.
func TestExternal_aNameThatIsThereButIsNoProgramIsPermissionDenied(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "data"), []byte("just words\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ script, first, hint string }{
		{script: "./data\necho \"st=$?\"\n", first: "./data: Permission denied", hint: "the file is there"},
		{script: "./sub\necho \"st=$?\"\n", first: "./sub: Permission denied", hint: "./sub is a directory"},
	} {
		t.Run(test.first, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr},
				runtime.State{Cwd: runtime.WorkingDirectory(dir)})

			// When
			rt.RunScript(context.Background(), test.script)

			// Then
			lines := strings.SplitN(stderr.String(), "\n", 2)
			if stdout.String() != "st=126\n" || !strings.HasSuffix(lines[0], test.first) || !strings.Contains(stderr.String(), test.hint) {
				t.Fatalf("stdout %q, stderr %q, want st=126, %q and a hint naming %q", stdout.String(), stderr.String(), test.first, test.hint)
			}
		})
	}
}
