//go:build !windows

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	shellruntime "github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func TestCommand_defaultDirectAppletRejectsRelativeAmbientTmpBeforeRun(t *testing.T) {
	t.Setenv("TMPDIR", "relative-tmp")
	for _, args := range [][]string{{"nemosh", "host-state-probe"}, {"host-state-probe"}} {
		t.Run(args[0], func(t *testing.T) {
			probe := &hostStateProbeApplet{}
			var stdout, stderr bytes.Buffer
			cmd := command{
				stdin:    &bytes.Buffer{},
				stdout:   &stdout,
				stderr:   &stderr,
				registry: applets.NewRegistry(probe),
			}

			err := cmd.run(context.Background(), args)

			if !errors.Is(err, shellruntime.ErrStateTmpRootNotAbsolute) {
				t.Fatalf("run(%v): got error %v, want %v", args, err, shellruntime.ErrStateTmpRootNotAbsolute)
			}
			if probe.called {
				t.Fatalf("run(%v): applet was called", args)
			}
			if stdout.Len() != 0 || stderr.Len() != 0 {
				t.Fatalf("run(%v): stdout=%q stderr=%q", args, stdout.String(), stderr.String())
			}
		})
	}
}

func TestCommand_defaultScriptModesRejectRelativeAmbientTmpBeforeRedirection(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "command string", args: []string{"nemosh", "-c"}},
		{name: "redirected stdin", args: []string{"nemosh"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "should-not-exist")
			script := "echo effect >" + filepath.ToSlash(target) + "\n"
			args := append([]string(nil), test.args...)
			stdin := &bytes.Buffer{}
			if len(args) > 1 {
				args = append(args, script)
			} else {
				stdin.WriteString(script)
			}
			t.Setenv("TMPDIR", "relative-tmp")
			var stdout, stderr bytes.Buffer
			cmd := command{stdin: stdin, stdout: &stdout, stderr: &stderr}

			err := cmd.run(context.Background(), args)

			if got, ok := err.(exitStatus); !ok || got != 1 {
				t.Fatalf("run(%v): got error %v, want exit status 1", args, err)
			}
			if stdout.Len() != 0 || !strings.Contains(stderr.String(), shellruntime.ErrStateTmpRootNotAbsolute.Error()) {
				t.Fatalf("run(%v): stdout=%q stderr=%q", args, stdout.String(), stderr.String())
			}
			if _, statErr := os.Stat(target); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("run(%v): effect path error=%v, want not exist", args, statErr)
			}
		})
	}
}

type hostStateProbeApplet struct {
	called bool
}

func (a *hostStateProbeApplet) Name() string {
	return "host-state-probe"
}

func (a *hostStateProbeApplet) Run(_ context.Context, _ []string, _ io.Reader, stdout, _ io.Writer) error {
	a.called = true
	_, err := io.WriteString(stdout, "effect\n")
	return err
}
