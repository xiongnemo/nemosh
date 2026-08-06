package applets

import (
	"errors"
	"io"
)

func newTestApplet(name string) Applet {
	return simpleApplet{name: name, run: func(args []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		if name == "[" {
			if !closedBracket(args) {
				// test_main2 strips argv[0] and then demands the last remaining
				// word be a lone "]", printing "missing ]" and returning 2 when
				// it is not (coreutils/test.c:897-901). With no operands the
				// check still runs and argv[0] itself is what fails it.
				return ExitStatusMessage(2, errors.New("missing ]"))
			}
			args = args[:len(args)-1]
		}
		if evalTest(args) {
			return nil
		}
		return ErrExitFalse
	}}
}

func closedBracket(args []string) bool {
	return len(args) > 0 && args[len(args)-1] == "]"
}

func evalTest(args []string) bool {
	switch len(args) {
	case 0:
		return false
	case 1:
		return args[0] != ""
	case 2:
		switch args[0] {
		case "-n":
			return args[1] != ""
		case "-z":
			return args[1] == ""
		default:
			return false
		}
	case 3:
		switch args[1] {
		case "=":
			return args[0] == args[2]
		case "!=":
			return args[0] != args[2]
		default:
			return false
		}
	default:
		return false
	}
}
