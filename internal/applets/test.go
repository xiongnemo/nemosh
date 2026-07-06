package applets

import "io"

func newTestApplet(name string) Applet {
	return simpleApplet{name: name, run: func(args []string, _ io.Reader, _ io.Writer, _ io.Writer) error {
		if name == "[" {
			if len(args) == 0 || args[len(args)-1] != "]" {
				return ErrExitFalse
			}
			args = args[:len(args)-1]
		}
		if evalTest(args) {
			return nil
		}
		return ErrExitFalse
	}}
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
