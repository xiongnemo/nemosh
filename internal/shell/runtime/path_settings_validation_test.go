package runtime

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestNewRuntimeWithState_rejectsInvalidEnabledTmpRoot(t *testing.T) {
	tests := []struct {
		name    string
		tmpRoot WorkingDirectory
		wantErr error
	}{
		{name: "missing", wantErr: ErrStateTmpRootRequired},
		{name: "relative", tmpRoot: "relative-tmp", wantErr: ErrStateTmpRootNotAbsolute},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			settings := DefaultPathSettings()
			settings.TmpRoot = test.tmpRoot

			_, err := NewRuntimeWithState(applets.DefaultRegistry, Streams{}, State{
				Cwd:   WorkingDirectory(t.TempDir()),
				Paths: &settings,
			})

			if !errors.Is(err, test.wantErr) {
				t.Fatalf("construct runtime: got %v, want %v", err, test.wantErr)
			}
		})
	}
}

func TestNewRuntimeWithState_acceptsInvalidTmpRoot_whenTmpIsDisabled(t *testing.T) {
	tests := []struct {
		name    string
		tmpRoot WorkingDirectory
	}{
		{name: "missing"},
		{name: "relative", tmpRoot: "relative-tmp"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			settings := DefaultPathSettings()
			settings.Config.EnableTmp = false
			settings.TmpRoot = test.tmpRoot

			_, err := NewRuntimeWithState(applets.DefaultRegistry, Streams{}, State{
				Cwd:   WorkingDirectory(t.TempDir()),
				Paths: &settings,
			})

			if err != nil {
				t.Fatalf("construct runtime with disabled tmp: %v", err)
			}
		})
	}
}

func TestNewRuntimeWithState_validatesCwdBeforeTmpRoot(t *testing.T) {
	settings := DefaultPathSettings()
	settings.TmpRoot = "relative-tmp"
	tests := []struct {
		name    string
		cwd     WorkingDirectory
		wantErr error
	}{
		{name: "missing", wantErr: ErrStateCwdRequired},
		{name: "relative", cwd: "relative-cwd", wantErr: ErrStateCwdNotAbsolute},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewRuntimeWithState(applets.DefaultRegistry, Streams{}, State{
				Cwd:   test.cwd,
				Paths: &settings,
			})

			if !errors.Is(err, test.wantErr) {
				t.Fatalf("construct runtime: got %v, want %v", err, test.wantErr)
			}
		})
	}
}

func TestNewRuntimeWithState_acceptsAbsoluteTmpRoot(t *testing.T) {
	settings := DefaultPathSettings()
	settings.TmpRoot = WorkingDirectory(t.TempDir())

	_, err := NewRuntimeWithState(applets.DefaultRegistry, Streams{}, State{
		Cwd:   WorkingDirectory(t.TempDir()),
		Paths: &settings,
	})

	if err != nil {
		t.Fatalf("construct runtime: %v", err)
	}
}

func TestNewWithState_failsBeforeEffects_whenEnabledTmpRootIsRelative(t *testing.T) {
	cwd := t.TempDir()
	target := filepath.Join(t.TempDir(), "should-not-exist")
	settings := DefaultPathSettings()
	settings.TmpRoot = "relative-tmp"
	var stderr bytes.Buffer
	runtime := NewWithState(applets.DefaultRegistry, Streams{Stderr: &stderr}, State{
		Cwd:   WorkingDirectory(cwd),
		Env:   NewEnvironment([]string{"PRESERVED=value"}),
		Paths: &settings,
	})

	_, resolveErr := runtime.ResolveNemoshPath("/tmp/effect")
	status := runtime.RunScript(context.Background(), "echo effect >"+filepath.ToSlash(target)+"\n")

	if !errors.Is(resolveErr, ErrStateTmpRootNotAbsolute) {
		t.Fatalf("resolve tmp path: got %v, want %v", resolveErr, ErrStateTmpRootNotAbsolute)
	}
	if status != 1 {
		t.Fatalf("status: got %d, want 1", status)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("effect path: got %v, want not exist", err)
	}
	if value, ok := runtime.LookupEnv("PRESERVED"); !ok || value != "value" {
		t.Fatalf("environment: value=%q present=%t", value, ok)
	}
	if got := runtime.WorkingDirectory(); got != "" {
		t.Fatalf("working directory: got %q, want empty", got)
	}
	if !strings.Contains(stderr.String(), ErrStateTmpRootNotAbsolute.Error()) {
		t.Fatalf("stderr: %q", stderr.String())
	}
}
