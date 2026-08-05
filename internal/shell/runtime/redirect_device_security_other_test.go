//go:build !windows

package runtime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestFinalSecurity_disabledDevicesUseNativeHostRedirects(t *testing.T) {
	// Given
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("locate test executable: %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable,
		"-test.run=^TestFinalSecurity_disabledDevicesUseNativeHostRedirectsHelper$",
		"--", "nemosh-disabled-redirect-helper",
	)
	environment := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "NEMOSH_DISABLED_REDIRECT_HELPER=") {
			environment = append(environment, entry)
		}
	}
	command.Env = append(environment, "NEMOSH_DISABLED_REDIRECT_HELPER=1")
	command.Stdin = strings.NewReader("pipe-backed stdin")

	// When
	output, err := command.CombinedOutput()

	// Then
	if ctx.Err() != nil {
		t.Fatalf("disabled redirect helper timed out: %v", ctx.Err())
	}
	if err != nil {
		t.Fatalf("disabled redirect helper: %v output=%q", err, output)
	}
	if got, want := string(output), "native-host-redirects"; got != want {
		t.Fatalf("disabled redirect helper output: got %q want %q", got, want)
	}
}

func TestFinalSecurity_disabledDevicesUseNativeHostRedirectsHelper(t *testing.T) {
	if os.Getenv("NEMOSH_DISABLED_REDIRECT_HELPER") != "1" || !hasArgument("nemosh-disabled-redirect-helper") {
		return
	}
	// Given
	settings := DefaultPathSettings()
	settings.Config.EnableDev = false
	runtime := newOtherPathRuntimeWithSettings(mustOtherWorkingDirectory(t), settings)
	table := newFDTable(Streams{})

	// When
	inputErr := runtime.bindInputRedirect(table, redirectOperation{kind: redirectInput, target: 7, path: "/dev/stdin"})
	outputErr := runtime.bindOutputRedirect(table, redirectOperation{kind: redirectOutput, target: 8, path: "/dev/null"})

	// Then
	if inputErr != nil || outputErr != nil {
		t.Fatalf("disabled-device redirects: input=%v output=%v", inputErr, outputErr)
	}
	inputEntry, err := table.lookup(7)
	if err != nil {
		t.Fatalf("lookup native input: %v", err)
	}
	if file, ok := inputEntry.description.closer.(*os.File); !ok || file.Name() != "/dev/stdin" {
		t.Fatalf("native input closer: got %T name=%v", inputEntry.description.closer, inputEntry.description.closer)
	}
	outputEntry, err := table.lookup(8)
	if err != nil {
		t.Fatalf("lookup native output: %v", err)
	}
	if file, ok := outputEntry.description.closer.(*os.File); !ok || file.Name() != "/dev/null" {
		t.Fatalf("native output closer: got %T name=%v", outputEntry.description.closer, outputEntry.description.closer)
	}
	if err := table.closeAll(); err != nil {
		t.Fatalf("close native redirects: %v", err)
	}
	if _, err := fmt.Fprint(os.Stdout, "native-host-redirects"); err != nil {
		t.Fatalf("write helper output: %v", err)
	}
	os.Exit(0)
}
