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

func TestRuntime_executesNestedCompoundsOnce_withoutLeakingClosers(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "nested if",
			source: "if true\nthen\necho outer-before\nif true\nthen\necho inner\nfi\necho outer-after\nfi\necho after\n",
			want:   "outer-before\ninner\nouter-after\nafter\n",
		},
		{
			name:   "nested loops",
			source: "for outer in one two\ndo\nfor inner in a b\ndo\necho $outer-$inner\ndone\ndone\necho after\n",
			want:   "one-a\none-b\ntwo-a\ntwo-b\nafter\n",
		},
		{
			name:   "if in loop",
			source: "for item in one two\ndo\nif true\nthen\necho $item\nfi\ndone\necho after\n",
			want:   "one\ntwo\nafter\n",
		},
		{
			name:   "loop in if",
			source: "if true\nthen\nfor item in one two\ndo\necho $item\ndone\nfi\necho after\n",
			want:   "one\ntwo\nafter\n",
		},
		{
			name:   "nested case",
			source: "case outer in\nouter)\ncase inner in\ninner)\necho nested\n;;\nesac\n;;\nesac\necho after\n",
			want:   "nested\nafter\n",
		},
		{
			name:   "case containing compounds",
			source: "case selected in\nselected)\nif true\nthen\nfor item in one two\ndo\necho $item\ndone\nfi\n;;\nesac\necho after\n",
			want:   "one\ntwo\nafter\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})

			// When
			status := rt.RunScript(context.Background(), test.source)

			// Then
			if status != 0 {
				t.Fatalf("RunScript() status = %d, want 0; stderr = %q", status, stderr.String())
			}
			if got := stdout.String(); got != test.want {
				t.Fatalf("RunScript() stdout = %q, want %q; stderr = %q", got, test.want, stderr.String())
			}
		})
	}
}

func TestRuntime_propagatesFlowAcrossNestedCompounds(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		wantStatus int
		wantOutput string
	}{
		{
			name:       "break",
			source:     "for item in one two\ndo\nif true\nthen\necho $item\nbreak\nfi\necho bad\ndone\necho after\n",
			wantOutput: "one\nafter\n",
		},
		{
			name:       "continue",
			source:     "for item in one two\ndo\nif true\nthen\necho $item\ncontinue\nfi\necho bad\ndone\necho after\n",
			wantOutput: "one\ntwo\nafter\n",
		},
		{
			name:       "exit",
			source:     "if true\nthen\ncase x in\nx)\nexit 7\n;;\nesac\nfi\necho bad\n",
			wantStatus: 7,
		},
		{
			name:       "exec",
			source:     "trap 'echo bad-trap' EXIT\nif true\nthen\nfor item in one\ndo\nexec echo replaced\ndone\nfi\necho bad\n",
			wantOutput: "replaced\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Given
			var stdout bytes.Buffer
			rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

			// When
			status := rt.RunScript(context.Background(), test.source)

			// Then
			if status != test.wantStatus {
				t.Fatalf("RunScript() status = %d, want %d", status, test.wantStatus)
			}
			if got := stdout.String(); got != test.wantOutput {
				t.Fatalf("RunScript() stdout = %q, want %q", got, test.wantOutput)
			}
		})
	}
}

func TestRuntime_propagatesReturnAcrossNestedCompounds(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	dir := t.TempDir()
	scriptPath := filepath.ToSlash(filepath.Join(dir, "nested-return.sh"))
	source := "if true\nthen\ncase x in\nx)\nreturn 9\n;;\nesac\nfi\necho bad\n"
	if err := os.WriteFile(scriptPath, []byte(source), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout})

	// When
	status := rt.RunScript(context.Background(), ". "+scriptPath+" || echo returned\necho after\n")

	// Then
	if status != 0 {
		t.Fatalf("RunScript() status = %d, want 0", status)
	}
	if got, want := stdout.String(), "returned\nafter\n"; got != want {
		t.Fatalf("RunScript() stdout = %q, want %q", got, want)
	}
}
