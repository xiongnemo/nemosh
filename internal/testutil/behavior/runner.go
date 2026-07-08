package behavior

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/xiongnemo/nemosh/internal/applets"
)

type Runner struct {
	registry applets.Registry
}

type Result struct {
	Status int
	Stdout string
	Stderr string
}

func NewRunner(registry applets.Registry) Runner {
	return Runner{registry: registry}
}

func (r Runner) Run(ctx context.Context, c Case) Result {
	if len(c.Command) == 0 {
		return Result{Status: 2, Stderr: "behavior case has no command\n"}
	}
	name := c.Command[0]
	applet, ok := r.registry.Lookup(name)
	if !ok {
		return Result{Status: 127, Stderr: fmt.Sprintf("%s: not found\n", name)}
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := applet.Run(ctx, c.Command[1:], bytes.NewReader(nil), &stdout, &stderr)
	return Result{Status: statusFromError(err), Stdout: stdout.String(), Stderr: stderr.String()}
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
