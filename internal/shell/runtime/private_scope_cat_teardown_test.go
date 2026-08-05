package runtime_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/applets"
	shellruntime "github.com/xiongnemo/nemosh/internal/shell/runtime"
)

const privateCatHelperEnv = "NEMOSH_PRIVATE_CAT_HELPER_PROCESS"

func TestPrivateScopeCatHelperProcess(t *testing.T) {
	if os.Getenv(privateCatHelperEnv) != "1" {
		return
	}

	fmt.Fprintln(os.Stdout, "READY")
	rt := shellruntime.New(applets.DefaultRegistry, shellruntime.Streams{})
	status := rt.RunScript(context.Background(), "(cat < /dev/zero > /dev/null &)\n")
	fmt.Fprintf(os.Stdout, "RETURN %d\n", status)
	os.Exit(status)
}

func TestRuntime_privateSubshellScopeTeardownStopsInProcessCat(t *testing.T) {
	// Given
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestPrivateScopeCatHelperProcess$")
	cmd.Env = append(os.Environ(), privateCatHelperEnv+"=1")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// When
	err := cmd.Run()

	// Then
	if ctx.Err() != nil {
		t.Fatalf("private-scope helper exceeded deadline: %v; protocol=%q stderr=%q", ctx.Err(), stdout.String(), stderr.String())
	}
	if err != nil {
		t.Fatalf("private-scope helper failed: %v; protocol=%q stderr=%q", err, stdout.String(), stderr.String())
	}
	if got, want := stdout.String(), "READY\nRETURN 0\n"; got != want {
		t.Fatalf("helper protocol = %q, want %q; stderr=%q", got, want, stderr.String())
	}
}
