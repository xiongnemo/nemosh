package runtime

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// sh and bash that PATH has no program for are this shell: `sh -c` runs, `bash -c` runs, and
// `command -v` names the binary that runs them. Each was `not found`, 127, as any other name
// PATH has nothing for still is.
func TestShellFallback_runsThisShellForShAndBash(t *testing.T) {
	self, err := jobExecutable()
	if err != nil {
		t.Skip("no binary of this shell to fall back to")
	}
	t.Setenv("PATH", t.TempDir())
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(),
		"sh -c 'exit 33'; echo \"sh $?\"\nbash -c 'echo bash $((1+2))'\ncommand -v sh\nnemosh-no-such-fallback 2>/dev/null; echo \"other $?\"\n")

	// Then
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if status != 0 || len(lines) != 4 || lines[0] != "sh 33" || lines[1] != "bash 3" || lines[3] != "other 127" ||
		!strings.EqualFold(filepath.ToSlash(lines[2]), filepath.ToSlash(self)) {
		t.Fatalf("status %d, stdout %q, stderr %q; want sh 33, bash 3, %s and other 127", status, stdout.String(), stderr.String(), self)
	}
}
