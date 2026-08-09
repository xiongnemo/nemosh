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
		options, operands, err := parseAppletOptions(args, "a", "")
		if err != nil {
			return err
		}
		if len(operands) == 0 {
			return missingOperand()
		}
		// -a makes every operand a path to strip, where without it the second
		// operand is a suffix to remove from the first. The two readings are
		// exclusive, which is why the option exists: `basename a b` means one
		// thing and `basename -a a b` means another.
		if options.has('a') {
			for _, operand := range operands {
				fmt.Fprintln(stdout, baseName(operand))
			}
			return nil
		}
		name := baseName(operands[0])
		if len(operands) > 1 {
			name = strings.TrimSuffix(name, operands[1])
		}
		fmt.Fprintln(stdout, name)
		return nil
	}}
}

// baseName is the last component, with a Windows separator treated as one too.
// A path typed on Windows arrives with either, and answering `C:\a\b` with the
// whole string would be answering about a filename nobody has.
func baseName(operand string) string {
	return path.Base(strings.ReplaceAll(operand, "\\", "/"))
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
