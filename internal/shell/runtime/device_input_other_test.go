//go:build !windows

package runtime

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestP05WaveA_OpenProcessInput_opensNativeDevStdinAsHostFile_whenDevicesDisabled(t *testing.T) {
	// Given
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("locate test executable: %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable,
		"-test.run=^TestP05WaveA_OpenProcessInput_disabledDevStdinHostHelper$",
		"--", "nemosh-disabled-dev-stdin-helper",
	)
	environment := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "NEMOSH_DISABLED_DEV_STDIN_HELPER=") {
			environment = append(environment, entry)
		}
	}
	command.Env = append(environment, "NEMOSH_DISABLED_DEV_STDIN_HELPER=1")
	command.Stdin = strings.NewReader("pipe-backed stdin")

	// When
	output, err := command.CombinedOutput()

	// Then
	if ctx.Err() != nil {
		t.Fatalf("disabled-device helper timed out: %v", ctx.Err())
	}
	if err != nil {
		t.Fatalf("disabled-device helper: %v output=%q", err, output)
	}
	if got, want := string(output), "pipe-backed stdin"; got != want {
		t.Fatalf("disabled-device helper output: got %q want %q", got, want)
	}
}

func TestP05WaveA_OpenProcessInput_disabledDevStdinHostHelper(t *testing.T) {
	if os.Getenv("NEMOSH_DISABLED_DEV_STDIN_HELPER") != "1" || !hasArgument("nemosh-disabled-dev-stdin-helper") {
		return
	}
	settings := DefaultPathSettings()
	settings.Config.EnableDev = false
	runtime := newOtherPathRuntimeWithSettings(mustOtherWorkingDirectory(t), settings)
	input, err := runtime.OpenProcessInput("/dev/stdin")
	if err != nil {
		t.Fatalf("open native /dev/stdin: %v", err)
	}
	file, ok := input.(*os.File)
	if !ok {
		t.Fatalf("native /dev/stdin input type: got %T want *os.File", input)
	}
	if file.Name() != "/dev/stdin" {
		t.Fatalf("native /dev/stdin name: got %q want %q", file.Name(), "/dev/stdin")
	}
	contents, readErr := io.ReadAll(input)
	closeErr := input.Close()
	if readErr != nil {
		t.Fatalf("read native /dev/stdin: %v", readErr)
	}
	if closeErr != nil {
		t.Fatalf("close native /dev/stdin: %v", closeErr)
	}
	if _, err := io.Copy(os.Stdout, bytes.NewReader(contents)); err != nil {
		t.Fatalf("write helper output: %v", err)
	}
	os.Exit(0)
}

func hasArgument(want string) bool {
	return slices.Contains(os.Args, want)
}

func TestP05WaveA_OpenProcessInput_opensAndClosesResolvedNativeHostFile(t *testing.T) {
	// Given
	directory := t.TempDir()
	path := filepath.Join(directory, "host-input.txt")
	if err := os.WriteFile(path, []byte("host contents"), 0o600); err != nil {
		t.Fatalf("write host input: %v", err)
	}
	settings := DefaultPathSettings()
	settings.Config.EnableDev = false
	runtime := NewWithState(applets.DefaultRegistry, Streams{}, State{
		Cwd:   WorkingDirectory(directory),
		Env:   NewEnvironment(nil),
		Paths: &settings,
	})

	// When
	input, err := runtime.OpenProcessInput("host-input.txt")
	if err != nil {
		t.Fatalf("open host input: %v", err)
	}
	contents, readErr := io.ReadAll(input)
	closeErr := input.Close()

	// Then
	if readErr != nil {
		t.Fatalf("read host input: %v", readErr)
	}
	if got, want := string(contents), "host contents"; got != want {
		t.Fatalf("host input contents: got %q want %q", got, want)
	}
	if closeErr != nil {
		t.Fatalf("close host input: %v", closeErr)
	}
}
