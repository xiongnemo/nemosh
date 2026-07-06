package applets

import (
	"io"
	"os"
)

func newTouchApplet() Applet {
	return simpleApplet{name: "touch", run: func(args []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		if len(args) == 0 {
			return ErrExitFalse
		}
		for _, path := range args {
			file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o666)
			if err != nil {
				return err
			}
			if err := file.Close(); err != nil {
				return err
			}
		}
		return nil
	}}
}

func newRmApplet() Applet {
	return simpleApplet{name: "rm", run: func(args []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		if len(args) == 0 {
			return ErrExitFalse
		}
		for _, path := range args {
			if err := os.Remove(path); err != nil {
				return err
			}
		}
		return nil
	}}
}

func newMkdirApplet() Applet {
	return simpleApplet{name: "mkdir", run: func(args []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		if len(args) == 0 {
			return ErrExitFalse
		}
		for _, path := range args {
			if err := os.Mkdir(path, 0o777); err != nil {
				return err
			}
		}
		return nil
	}}
}

func newRmdirApplet() Applet {
	return simpleApplet{name: "rmdir", run: func(args []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		if len(args) == 0 {
			return ErrExitFalse
		}
		for _, path := range args {
			if err := os.Remove(path); err != nil {
				return err
			}
		}
		return nil
	}}
}
