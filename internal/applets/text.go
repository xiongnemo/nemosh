package applets

import (
	"fmt"
	"io"
	"path"
	"strings"
)

// `basename -z /a/b` printed `-z`: the flag was taken as the operand, so the
// applet answered a question nobody asked and reported success.
func newBasenameApplet() Applet {
	return simpleApplet{name: "basename", run: func(args []string, _ io.Reader, stdout, _ io.Writer) error {
		_, operands, err := parseAppletOptions(args, "", "")
		if err != nil {
			return err
		}
		if len(operands) == 0 {
			return missingOperand()
		}
		name := path.Base(strings.ReplaceAll(operands[0], "\\", "/"))
		if len(operands) > 1 {
			name = strings.TrimSuffix(name, operands[1])
		}
		fmt.Fprintln(stdout, name)
		return nil
	}}
}

func newDirnameApplet() Applet {
	return simpleApplet{name: "dirname", run: func(args []string, _ io.Reader, stdout, _ io.Writer) error {
		_, operands, err := parseAppletOptions(args, "", "")
		if err != nil {
			return err
		}
		if len(operands) == 0 {
			return missingOperand()
		}
		dir := path.Dir(strings.ReplaceAll(operands[0], "\\", "/"))
		fmt.Fprintln(stdout, dir)
		return nil
	}}
}
