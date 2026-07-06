package applets

import (
	"fmt"
	"io"
	"path"
	"strings"
)

func newPrintfApplet() Applet {
	return simpleApplet{name: "printf", run: func(args []string, _ io.Reader, stdout, _ io.Writer) error {
		if len(args) == 0 {
			return nil
		}
		format := strings.ReplaceAll(args[0], "\\n", "\n")
		values := make([]any, 0, len(args)-1)
		for _, arg := range args[1:] {
			values = append(values, arg)
		}
		_, err := fmt.Fprintf(stdout, format, values...)
		return err
	}}
}

func newBasenameApplet() Applet {
	return simpleApplet{name: "basename", run: func(args []string, _ io.Reader, stdout, _ io.Writer) error {
		if len(args) == 0 {
			return ErrExitFalse
		}
		name := path.Base(strings.ReplaceAll(args[0], "\\", "/"))
		if len(args) > 1 {
			name = strings.TrimSuffix(name, args[1])
		}
		fmt.Fprintln(stdout, name)
		return nil
	}}
}

func newDirnameApplet() Applet {
	return simpleApplet{name: "dirname", run: func(args []string, _ io.Reader, stdout, _ io.Writer) error {
		if len(args) == 0 {
			return ErrExitFalse
		}
		dir := path.Dir(strings.ReplaceAll(args[0], "\\", "/"))
		fmt.Fprintln(stdout, dir)
		return nil
	}}
}
