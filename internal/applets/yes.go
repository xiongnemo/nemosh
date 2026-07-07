package applets

import (
	"context"
	"fmt"
	"io"
	"strings"
)

func newYesApplet() Applet {
	return yesApplet{}
}

type yesApplet struct{}

func (yesApplet) Name() string {
	return "yes"
}

func (yesApplet) Run(ctx context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
	line := "y"
	if len(args) > 0 {
		line = strings.Join(args, " ")
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if _, err := fmt.Fprintln(stdout, line); err != nil {
			return err
		}
	}
}
