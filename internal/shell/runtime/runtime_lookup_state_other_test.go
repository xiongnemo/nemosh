//go:build !windows

package runtime_test

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_externalLookupSearchesPATH_forQuotedBackslashNameOnNonWindows(t *testing.T) {
	command := `nemosh\backslash-name`
	bin := t.TempDir()
	executable := filepath.Join(bin, command)
	copyRuntimeHelper(t, executable)
	var stdout bytes.Buffer
	rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout}, runtime.State{
		Cwd: runtime.WorkingDirectory(t.TempDir()),
		Env: runtime.NewEnvironment([]string{"PATH=" + bin, "NEMOSH_RUNTIME_HELPER_PROCESS=1"}),
	})

	status := rt.RunScript(context.Background(), shellSingleQuote(command)+" -test.run=TestRuntimeHelperProcess -- executable\n")

	if status != 0 {
		t.Fatalf("status: got %d, want 0", status)
	}
	if got := strings.TrimSpace(stdout.String()); !sameNativePath(got, executable) {
		t.Fatalf("executable: got %q, want %q", got, executable)
	}
}
