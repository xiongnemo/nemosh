package applets

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
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
	// `bc prelude.bc` a session rather than a batch job.
	bad, err := in.runSession(stdin)
	if bad {
		failed = true
	}
	if err != nil {
		// Returned rather than printed. Every other applet returns what its scanner
		// failed with, and the shell prints `bc: <err>` from it -- the same words this
		// used to print itself -- so nothing is lost for a real failure. What is gained
		// is the cancelled case: runCommand answers 130 and says nothing when the error
		// it gets back is the context's, and printing it instead put `bc: context
		// canceled` in front of someone who had pressed Ctrl-C.
		return err
	}
	return bcStatus(failed)
}

// runSession reads standard input **a line at a time**, running each statement as it
// completes.
//
// Reading to the end first and then running the lot is what a batch job wants, and it is
// what this did -- which meant an interactive `bc` printed nothing and appeared to hang,
// because a terminal has no end. It also meant one typo anywhere threw away the whole
// session's worth of input.
//
// A line that cannot be parsed *yet* is held and the next one added to it, which is how
// `define f(n) {` on its own line works. Which failures mean "not yet" is decided by the
// parser and measured against the references -- see errBcIncomplete.
func (in *bcInterp) runSession(stdin io.Reader) (bool, error) {
	reader := bufio.NewScanner(decodeTextInput(stdin))
	reader.Buffer(make([]byte, 0, 64*1024), maxTextLine)
	failed := false
	var pending strings.Builder
	for reader.Scan() {
		pending.WriteString(reader.Text())
		pending.WriteString("\n")
		program, err := parseBcProgram(pending.String())
		if errors.Is(err, errBcIncomplete) {
			// Still open: a brace, or a construct whose body is on the next line.
			continue
		}
		pending.Reset()
		if err != nil {
			in.report(err)
			failed = true
			continue
		}
		if bad := in.runParsed(program); bad {
			failed = true
		}
		// Flushed per statement, because the answer is the point of typing the line.
		in.out.Flush()
		if in.halted {
			return failed, nil
		}
	}
	if strings.TrimSpace(pending.String()) != "" {
		// The input ended in the middle of something, which is an error rather than an
		// invitation: there is no next line coming.
		in.report(fmt.Errorf("unexpected end of input"))
		failed = true
	}
	if err := reader.Err(); err != nil {
		return failed, err
	}
	return failed, nil
}

// runParsed runs an already-parsed program, reporting whether anything went wrong.
func (in *bcInterp) runParsed(program []bcStmt) bool {
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
	return in.runParsed(program)
}
