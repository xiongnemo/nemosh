package applets

import (
	"fmt"
	"io"
)

// bc, the calculator language.
//
// The whole POSIX language: variables, arrays, `if`, `while`, `for`, `define` with autos and
// recursion, `print`, and arithmetic at whatever precision `scale` asks for. The numbers are
// exact -- see decimal.go for what that means and why it is not a float.
//
// What is **not** here, and is refused by name rather than approximated:
//
//   - **`-l`, the maths library** (`s`, `c`, `a`, `l`, `e`, `j`). Those are series
//     expansions, and a wrong one is wrong in the last digits of an answer that still looks
//     right -- the worst shape an error can take in a calculator. Better absent than
//     approximate.
//   - **`read()`**, which would make bc's own input and the program's input the same stream.
//   - **An output base above 16.** POSIX prints digits above that as space-separated decimal
//     groups, which is a different output format rather than a longer alphabet.
//
// A program may come from files, and standard input is read after them -- so
// `bc prelude.bc` still takes a session at the keyboard, which is how bc is used.
func newBcApplet() Applet {
	return simpleApplet{name: "bc", run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		files, err := parseBcArguments(args)
		if err != nil {
			return err
		}
		interp := newBcInterp(stdout, stderr)
		return interp.run(files, stdin)
	}}
}

// parseBcArguments reads the options, of which only one changes anything.
func parseBcArguments(args []string) ([]string, error) {
	var files []string
	for index := 0; index < len(args); index++ {
		argument := args[index]
		switch {
		case argument == "--":
			files = append(files, args[index+1:]...)
			index = len(args)
		case argument == "-q" || argument == "--quiet":
			// No banner is printed at all, so -q is already the behaviour.
		case argument == "-s" || argument == "--standard":
			// POSIX mode. What this implements *is* the POSIX language, so the flag asks
			// for what it already does.
		case argument == "-w" || argument == "--warn":
			// Warnings about non-POSIX constructs, of which this accepts almost none.
		case argument == "-l" || argument == "--mathlib":
			return nil, fmt.Errorf("-l is not supported: the maths library would be a series expansion, " +
				"and a wrong one is wrong in the last digits of an answer that still looks right")
		case len(argument) > 1 && argument[0] == '-':
			return nil, fmt.Errorf("unsupported option: %s", argument)
		default:
			files = append(files, argument)
		}
	}
	return files, nil
}

// run works through the files and then standard input.
func (in *bcInterp) run(files []string, stdin io.Reader) error {
	defer in.out.Flush()
	failed := false
	for _, name := range files {
		text, err := readAwkSource(name)
		if err != nil {
			return err
		}
		if bad := in.runSource(text); bad {
			failed = true
		}
		if in.halted {
			return bcStatus(failed)
		}
	}
	// Standard input is read even when files were given, which is what makes
	// `bc prelude.bc` a session rather than a batch job. A terminal is handled the same
	// way: the shell hands over whatever it has.
	text, err := io.ReadAll(decodeTextInput(stdin))
	if err != nil {
		return err
	}
	if len(text) > 0 {
		if bad := in.runSource(string(text)); bad {
			failed = true
		}
	}
	return bcStatus(failed)
}

func bcStatus(failed bool) error {
	if failed {
		return ErrExitFalse
	}
	return nil
}

// runSource parses and runs one program, reporting whether anything went wrong.
//
// **A parse failure stops that source and no more**, which is what makes an interactive
// session usable: a typo on one line should not end the session, and a file with a syntax
// error should not stop the files after it.
func (in *bcInterp) runSource(source string) bool {
	program, err := parseBcProgram(source)
	if err != nil {
		in.report(err)
		return true
	}
	flow, err := in.execBody(program)
	if err != nil {
		in.report(err)
		return true
	}
	if flow == bcFlowHalt {
		in.halted = true
	}
	return false
}
