package applets

import (
	"context"
	"errors"
	"io"
)

func newTestApplet(name string) Applet {
	return simpleApplet{name: name, runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
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
		evaluator := testEvaluator{
			args:    args,
			view:    ProcessViewFromContext(ctx),
			streams: [3]any{stdin, stdout, stderr},
		}
		result, err := evaluator.evaluate()
		if err != nil {
			return ExitStatusMessage(2, err)
		}
		if result {
			return nil
		}
		return ErrExitFalse
	}}
}

func closedBracket(args []string) bool {
	return len(args) > 0 && args[len(args)-1] == "]"
}
