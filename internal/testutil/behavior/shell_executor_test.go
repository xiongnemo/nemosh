package behavior_test

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/xiongnemo/nemosh/internal/testutil/behavior"
)

func newShellExecutor(binary string, timeout time.Duration) behavior.ScriptExecutor {
	return func(ctx context.Context, request behavior.ScriptRequest) (behavior.ProcessResult, error) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd := exec.CommandContext(ctx, binary, "-c", request.Script)
		cmd.Dir = request.Dir
		cmd.Env = request.Env
		cmd.Stdin = strings.NewReader(request.Stdin)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		if ctxErr := ctx.Err(); ctxErr != nil {
			return behavior.ProcessResult{}, ctxErr
		}
		status := 0
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			status = exitErr.ExitCode()
		} else if err != nil {
			return behavior.ProcessResult{}, err
		}
		return behavior.ProcessResult{Status: status, Stdout: stdout.String(), Stderr: stderr.String()}, nil
	}
}

func TestShellExecutor_returnsHarnessErrorWhenDeadlineExpires(t *testing.T) {
	// Given
	binary := buildFreshNemosh(t)
	executor := newShellExecutor(binary, time.Nanosecond)

	// When
	result, err := executor(t.Context(), behavior.ScriptRequest{Script: "echo unreachable"})

	// Then
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline harness error, got result %+v and error %v", result, err)
	}
}

func TestShellExecutor_returnsStatusWhenShellExitsNonzero(t *testing.T) {
	// Given
	binary := buildFreshNemosh(t)
	executor := newShellExecutor(binary, 10*time.Second)

	// When
	result, err := executor(t.Context(), behavior.ScriptRequest{Script: "exit 7"})

	// Then
	if err != nil {
		t.Fatalf("expected process result, got harness error %v", err)
	}
	if result.Status != 7 {
		t.Fatalf("expected status 7, got %d", result.Status)
	}
}
