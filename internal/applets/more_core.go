package applets

import (
	"context"
	"fmt"
	"io"
)

type pwdApplet struct{}

func newPwdApplet() Applet     { return pwdApplet{} }
func (pwdApplet) Name() string { return "pwd" }
func (pwdApplet) Run(ctx context.Context, _ []string, _ io.Reader, stdout, _ io.Writer) error {
	_, err := fmt.Fprintln(stdout, ProcessViewFromContext(ctx).WorkingDirectory())
	return err
}
