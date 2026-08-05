package runtime

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestRuntime_childExecutionCopiesDoNotOverwriteInteractiveStatus(t *testing.T) {
	tests := []struct {
		name string
		run  func(*testing.T, Runtime) int
	}{
		{name: "redirect", run: func(t *testing.T, r Runtime) int {
			path := filepath.ToSlash(filepath.Join(t.TempDir(), "out"))
			return r.runCommandWithRedirects(context.Background(), []string{"true", ">", path})
		}},
		{name: "pipeline", run: func(_ *testing.T, r Runtime) int {
			return r.runPipeline(context.Background(), []string{"true", "|", "true"})
		}},
		{name: "substitution", run: func(_ *testing.T, r Runtime) int {
			prepared, err := ParseScript("true")
			if err != nil {
				t.Fatal(err)
			}
			r.commandSubstitutionScript(context.Background(), prepared)
			return 0
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			rt := New(applets.DefaultRegistry, Streams{})
			script, err := ParseScript("false\n")
			if err != nil {
				t.Fatalf("ParseScript() error = %v", err)
			}
			rt.RunInteractive(context.Background(), script)

			// When
			childStatus := tt.run(t, rt)
			parentStatus := rt.CloseInteractive(context.Background())

			// Then
			if childStatus != 0 {
				t.Fatalf("child status = %d, want 0", childStatus)
			}
			if parentStatus != 1 {
				t.Fatalf("parent CloseInteractive() = %d, want 1", parentStatus)
			}
		})
	}
}
