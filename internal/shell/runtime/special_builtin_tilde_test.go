package runtime_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestRuntime_sourcesOnlyUnquotedLeadingTilde(t *testing.T) {
	// Given
	home := t.TempDir()
	cwd := t.TempDir()
	literalHome := filepath.Join(cwd, "~")
	if err := os.Mkdir(literalHome, 0o700); err != nil {
		t.Fatalf("create literal tilde directory: %v", err)
	}
	writeSourceMarker(t, filepath.Join(home, ".profile"), "home")
	writeSourceMarker(t, filepath.Join(literalHome, ".profile"), "literal")

	tests := []struct {
		name    string
		command string
		want    string
	}{
		{name: "source unquoted", command: "source ~/.profile", want: "home\n"},
		{name: "dot unquoted", command: ". ~/.profile", want: "home\n"},
		{name: "single quoted", command: "source '~/.profile'", want: "literal\n"},
		{name: "double quoted", command: `source "~/.profile"`, want: "literal\n"},
		{name: "escaped", command: `source \~/.profile`, want: "literal\n"},
		{name: "zero width single quote", command: `source ~''/.profile`, want: "literal\n"},
		{name: "zero width double quote", command: `source ~""/.profile`, want: "literal\n"},
		{name: "parameter generated", command: "path='~/.profile'\nsource \"$path\"", want: "literal\n"},
		{name: "command generated", command: `source "$(echo '~/.profile')"`, want: "literal\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			rt := runtime.NewWithState(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout}, runtime.State{
				Cwd: runtime.WorkingDirectory(cwd),
				Env: runtime.NewEnvironment([]string{"HOME=" + home}),
			})

			// When
			status := rt.RunScript(context.Background(), test.command+"\necho $marker\n")

			// Then
			if status != 0 {
				t.Fatalf("status = %d, want 0", status)
			}
			if got := stdout.String(); got != test.want {
				t.Fatalf("stdout = %q, want %q", got, test.want)
			}
		})
	}
}

func writeSourceMarker(t *testing.T, path, marker string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("marker="+marker+"\n"), 0o600); err != nil {
		t.Fatalf("write source marker %q: %v", path, err)
	}
}
