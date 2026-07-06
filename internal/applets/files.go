package applets

import (
	"io"
	"os"
)

func newCatApplet() Applet {
	return simpleApplet{name: "cat", run: func(args []string, stdin io.Reader, stdout, _ io.Writer) error {
		if len(args) == 0 {
			_, err := io.Copy(stdout, stdin)
			return err
		}
		for _, path := range args {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(stdout, file)
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		}
		return nil
	}}
}
