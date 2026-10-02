package applets

import (
	"context"
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

// yesBlock is how much yes hands its writer at a time: whole lines, as many as fit, as
// busybox's stdio hands its pipe a buffer's worth. One write a line made `yes | head -c 1000000`
// half a million writes, and took four times busybox's; a consumer sees the same lines.
const yesBlock = 8192

func (yesApplet) Run(ctx context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
	line := "y"
	if len(args) > 0 {
		line = strings.Join(args, " ")
	}
	unit := line + "\n"
	block := strings.Repeat(unit, max(1, yesBlock/len(unit)))
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if _, err := io.WriteString(stdout, block); err != nil {
			return err
		}
	}
}
