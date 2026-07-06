package applets

import (
	"fmt"
	"io"
	"os"
)

func newPwdApplet() Applet {
	return simpleApplet{name: "pwd", run: func(_ []string, _ io.Reader, stdout, _ io.Writer) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, cwd)
		return err
	}}
}
