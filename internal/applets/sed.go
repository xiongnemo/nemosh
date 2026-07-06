package applets

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

func newSedApplet() Applet {
	return simpleApplet{name: "sed", run: func(args []string, stdin io.Reader, stdout, _ io.Writer) error {
		if len(args) != 1 {
			return ErrExitFalse
		}
		old, replacement, err := parseSedSubstitute(args[0])
		if err != nil {
			return err
		}
		scanner := bufio.NewScanner(stdin)
		for scanner.Scan() {
			line := strings.Replace(scanner.Text(), old, replacement, 1)
			if _, err := fmt.Fprintln(stdout, line); err != nil {
				return err
			}
		}
		return scanner.Err()
	}}
}

func parseSedSubstitute(script string) (string, string, error) {
	if len(script) < 3 || script[0] != 's' {
		return "", "", fmt.Errorf("unsupported sed script: %s", script)
	}
	delim := script[1]
	if delim == 0 {
		return "", "", fmt.Errorf("unsupported sed script: %s", script)
	}
	parts := strings.SplitN(script[2:], string(delim), 3)
	if len(parts) != 3 || parts[0] == "" || parts[2] != "" {
		return "", "", fmt.Errorf("malformed sed substitute: %s", script)
	}
	return parts[0], parts[1], nil
}
