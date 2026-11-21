package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
)

func newTestApplet(name string) Applet {
	return simpleApplet{name: name, quiet: true, runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		// In bash mode, bash's: its reading of the arguments and its words; see test_bash.go.
		dialect := printfDialectOf(ctx)
		after := ""
		if name == "[" {
			if !closedBracket(args) {
				if dialect.bash {
					fmt.Fprintf(stderr, "%s[: %v\n", dialect.prefix, errBashMissingBracket)
					return ExitStatus(2)
				}
				// test_main2 strips argv[0] and then demands the last remaining
				// word be a lone "]", printing "missing ]" and returning 2 when
				// it is not (coreutils/test.c:897-901). With no operands the
				// check still runs and argv[0] itself is what fails it.
				return ExitStatusMessage(2, errors.New("missing ]"))
			}
			args, after = args[:len(args)-1], "]"
		}
		evaluator := testEvaluator{
			args:    args,
			view:    ProcessViewFromContext(ctx),
			streams: [3]any{stdin, stdout, stderr},
		}
		var result bool
		var err error
		if dialect.bash {
			result, err = (&bashTest{e: &evaluator, args: args, after: after}).run()
			if err != nil {
				fmt.Fprintf(stderr, "%s%s: %v\n", dialect.prefix, name, err)
				return ExitStatus(2)
			}
		} else if result, err = evaluator.evaluate(); err != nil {
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
