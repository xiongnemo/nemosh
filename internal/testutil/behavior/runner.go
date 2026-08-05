package behavior

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"runtime"

	"github.com/xiongnemo/nemosh/internal/applets"
)

type Runner struct {
	registry       applets.Registry
	scriptExecutor ScriptExecutor
}

type ScriptRequest struct {
	Script string
	Stdin  string
	Dir    string
	Env    []string
}

type ProcessResult struct {
	Status int
	Stdout string
	Stderr string
}

type ScriptExecutor func(context.Context, ScriptRequest) (ProcessResult, error)

type Result struct {
	Status       int
	Stdout       string
	Stderr       string
	HarnessError error
	SkipReason   string
}

func NewRunner(registry applets.Registry) Runner {
	return Runner{registry: registry}
}

func NewRunnerWithScriptExecutor(registry applets.Registry, executor ScriptExecutor) Runner {
	return Runner{registry: registry, scriptExecutor: executor}
}

func (r Runner) Run(ctx context.Context, c Case) Result {
	if reason := skipReason(c); reason != "" {
		return Result{SkipReason: reason}
	}
	if c.Script != "" {
		return r.runScript(ctx, c)
	}
	if len(c.Command) == 0 {
		return Result{HarnessError: errors.New("behavior case has no script or command")}
	}
	name := c.Command[0]
	applet, ok := r.registry.Lookup(name)
	if !ok {
		return Result{Status: 127, Stderr: fmt.Sprintf("%s: not found\n", name)}
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := applet.Run(ctx, c.Command[1:], bytes.NewReader([]byte(c.Stdin)), &stdout, &stderr)
	return Result{Status: statusFromError(err), Stdout: stdout.String(), Stderr: stderr.String()}
}

func skipReason(c Case) string {
	if len(c.Platforms) > 0 {
		matched := false
		for _, platform := range c.Platforms {
			matched = matched || platform == runtime.GOOS
		}
		if !matched {
			return fmt.Sprintf("platform %s is not enabled", runtime.GOOS)
		}
	}
	for _, requirement := range c.Requires {
		if requirement != runtime.GOOS {
			return fmt.Sprintf("unsupported requirement %q", requirement)
		}
	}
	return ""
}

func statusFromError(err error) int {
	if err == nil {
		return 0
	}
	if status, ok := applets.StatusCode(err); ok {
		return status
	}
	if errors.Is(err, applets.ErrExitFalse) {
		return 1
	}
	return 1
}
