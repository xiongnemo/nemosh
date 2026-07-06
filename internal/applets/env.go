package applets

import (
	"fmt"
	"io"
	"os"
)

func newEnvApplet() Applet {
	return simpleApplet{name: "env", run: func(args []string, _ io.Reader, stdout, _ io.Writer) error {
		if len(args) != 0 {
			return fmt.Errorf("env command execution is not implemented")
		}
		for _, item := range os.Environ() {
			fmt.Fprintln(stdout, item)
		}
		return nil
	}}
}

func newPrintenvApplet() Applet {
	return simpleApplet{name: "printenv", run: func(args []string, _ io.Reader, stdout, _ io.Writer) error {
		if len(args) == 0 {
			for _, item := range os.Environ() {
				fmt.Fprintln(stdout, item)
			}
			return nil
		}
		status := error(nil)
		for _, name := range args {
			value, ok := os.LookupEnv(name)
			if !ok {
				status = ErrExitFalse
				continue
			}
			fmt.Fprintln(stdout, value)
		}
		return status
	}}
}
