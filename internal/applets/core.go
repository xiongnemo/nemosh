package applets

import (
	"context"
	"fmt"
	"io"
	"strings"
)

type simpleApplet struct {
	name string
	run  func(args []string, stdin io.Reader, stdout, stderr io.Writer) error
}

func (a simpleApplet) Name() string {
	return a.name
}

func (a simpleApplet) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return a.run(args, stdin, stdout, stderr)
	}
}

func newTrueApplet() Applet {
	return simpleApplet{name: "true", run: func(_ []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		return nil
	}}
}

func newFalseApplet() Applet {
	return simpleApplet{name: "false", run: func(_ []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		return ErrExitFalse
	}}
}

func newEchoApplet() Applet {
	return simpleApplet{name: "echo", run: func(args []string, _ io.Reader, stdout, _ io.Writer) error {
		_, err := fmt.Fprintln(stdout, strings.Join(args, " "))
		return err
	}}
}
