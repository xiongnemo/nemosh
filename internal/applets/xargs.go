package applets

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
)

type xargsApplet struct{}

func newXargsApplet() Applet {
	return xargsApplet{}
}

func (xargsApplet) Name() string {
	return "xargs"
}

func (xargsApplet) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	commandArgs, err := xargsCommandArgs(args, stdin)
	if err != nil {
		return err
	}
	applet, ok := DefaultRegistry.Lookup(commandArgs[0])
	if !ok {
		return fmt.Errorf("%s: not found", commandArgs[0])
	}
	return applet.Run(ctx, commandArgs[1:], bytes.NewReader(nil), stdout, stderr)
}

func xargsCommandArgs(args []string, stdin io.Reader) ([]string, error) {
	commandArgs := []string{"echo"}
	if len(args) > 0 {
		if strings.HasPrefix(args[0], "-") {
			return nil, fmt.Errorf("unsupported xargs option: %s", args[0])
		}
		commandArgs = append([]string(nil), args...)
	}
	scanner := bufio.NewScanner(stdin)
	scanner.Split(bufio.ScanWords)
	for scanner.Scan() {
		commandArgs = append(commandArgs, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return commandArgs, nil
}
