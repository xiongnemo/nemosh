package applets

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// dc, the reverse-polish calculator, and the smaller of the two ways into decimal.go's
// arithmetic.
//
// It is a stack machine with one-character commands, which makes it terse to the point of
// unfriendliness -- and also makes it the thing a script reaches for when it needs exact
// arithmetic in one line: `echo "2 3 + p" | dc`.
//
// **A register is a whole stack, not a slot.** `sa` replaces register `a` and `Sa` pushes
// onto it, which is what makes dc able to express recursion with `[...]  x`. Both are here
// because the second is useless without the first.
//
// `!` -- dc's shell escape -- is **refused by name**. internal/applets does not spawn OS
// processes (docs/design/windows-execution-model.md), and a calculator that could run
// commands would be the most surprising thing in this shell.

// dcValue is what the stack holds: a number, or a string waiting to be executed or printed.
type dcValue struct {
	number bigDecimal
	text   string
	isText bool
}

type dcMachine struct {
	stack     []dcValue
	registers map[byte][]dcValue
	scale     int
	inputBase int
	outBase   int
	out       *bufio.Writer
	errors    io.Writer
	quit      bool
	// depth counts nested `x` executions so that `Q` can leave several at once.
	depth int
}

func newDcApplet() Applet {
	return simpleApplet{name: "dc", run: func(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		scripts, files, err := parseDcArguments(args)
		if err != nil {
			return err
		}
		return newDcMachine(stdout, stderr).run(scripts, files, stdin)
	}}
}

func newDcMachine(stdout, stderr io.Writer) *dcMachine {
	return &dcMachine{
		registers: map[byte][]dcValue{},
		inputBase: 10,
		outBase:   10,
		out:       bufio.NewWriter(stdout),
		errors:    stderr,
	}
}

// parseDcArguments reads `-e SCRIPT` and `-f FILE`, joined or separate, and the file
// operands.
func parseDcArguments(args []string) ([]string, []string, error) {
	var scripts, files []string
	for index := 0; index < len(args); index++ {
		argument := args[index]
		switch {
		case argument == "-x":
			// busybox's -x is "read the input as a script", which is what dc does anyway.
			continue
		case argument == "-e" || argument == "--expression":
			index++
			if index >= len(args) {
				return nil, nil, fmt.Errorf("-e needs a script")
			}
			scripts = append(scripts, args[index])
		case strings.HasPrefix(argument, "-e"):
			scripts = append(scripts, argument[2:])
		case argument == "-f" || argument == "--file":
			index++
			if index >= len(args) {
				return nil, nil, fmt.Errorf("-f needs a file")
			}
			files = append(files, args[index])
		case strings.HasPrefix(argument, "-f"):
			files = append(files, argument[2:])
		case argument == "--":
			files = append(files, args[index+1:]...)
			index = len(args)
		case len(argument) > 1 && argument[0] == '-':
			return nil, nil, fmt.Errorf("unsupported option: %s", argument)
		default:
			files = append(files, argument)
		}
	}
	return scripts, files, nil
}

// run works through the scripts and the files, flushing whatever was printed before it
// reports a failure so that the output and the diagnostic arrive in the order they happened.
func (m *dcMachine) run(scripts, files []string, stdin io.Reader) error {
	err := m.runAll(scripts, files, stdin)
	if flushErr := m.out.Flush(); flushErr != nil && err == nil {
		err = flushErr
	}
	if err != nil {
		fmt.Fprintf(m.errors, "dc: %v\n", err)
		return ErrExitFalse
	}
	return nil
}

func (m *dcMachine) runAll(scripts, files []string, stdin io.Reader) error {
	for _, script := range scripts {
		if err := m.execute(script); err != nil {
			return err
		}
		if m.quit {
			return nil
		}
	}
	if len(scripts) == 0 && len(files) == 0 {
		// No script and no file: the program is whatever arrives on standard input, read
		// **a line at a time**. Reading to the end first made an interactive dc print
		// nothing and appear to hang, because a terminal has no end.
		return m.session(stdin)
	}
	for _, name := range files {
		text, err := readAwkSource(name)
		if err != nil {
			return err
		}
		if err := m.execute(text); err != nil {
			return err
		}
		if m.quit {
			return nil
		}
	}
	return nil
}

// session runs standard input a line at a time, holding a line whose `[` is still open.
//
// Whether to hold is decided **before** anything runs, by counting brackets, rather than by
// running the line and seeing it fail. Running first and retrying the whole buffer would
// repeat whatever came before the `[`: `5 p [abc` would print 5, hold, and print it again
// when the next line closed the string.
func (m *dcMachine) session(stdin io.Reader) error {
	reader := bufio.NewScanner(decodeTextInput(stdin))
	reader.Buffer(make([]byte, 0, 64*1024), maxTextLine)
	var pending strings.Builder
	for reader.Scan() && !m.quit {
		pending.WriteString(reader.Text())
		pending.WriteString("\n")
		if dcOpenBrackets(pending.String()) > 0 {
			continue
		}
		source := pending.String()
		pending.Reset()
		if err := m.execute(source); err != nil {
			return err
		}
		// Flushed per line, because the answer is the point of typing it.
		if err := m.out.Flush(); err != nil {
			return err
		}
	}
	if strings.TrimSpace(pending.String()) != "" {
		// The input ended inside a string, and there is no next line coming.
		return errDcUnterminated
	}
	return reader.Err()
}

// dcOpenBrackets counts how many `[` are still open.
func dcOpenBrackets(source string) int {
	depth := 0
	for index := 0; index < len(source); index++ {
		switch source[index] {
		case '[':
			depth++
		case ']':
			if depth > 0 {
				depth--
			}
		}
	}
	return depth
}

// push and pop are the whole interface to the stack; everything else goes through them so
// that "too few elements" is reported in one place.
func (m *dcMachine) push(value dcValue) { m.stack = append(m.stack, value) }

func (m *dcMachine) pop() (dcValue, error) {
	if len(m.stack) == 0 {
		return dcValue{}, fmt.Errorf("stack has too few elements")
	}
	value := m.stack[len(m.stack)-1]
	m.stack = m.stack[:len(m.stack)-1]
	return value, nil
}

// popNumber takes a value and insists it is one, because `[text] 1 +` is a mistake rather
// than an arithmetic question.
func (m *dcMachine) popNumber() (bigDecimal, error) {
	value, err := m.pop()
	if err != nil {
		return bigDecimal{}, err
	}
	if value.isText {
		return bigDecimal{}, fmt.Errorf("a string is not a number")
	}
	return value.number, nil
}

// execute runs a stretch of dc source.
//
// **An error stops the script**, which was measured rather than assumed: `1 0 / 5 p` prints
// the diagnostic and then nothing, so the `5 p` never runs. Carrying on would leave the
// program working with a stack it did not expect, which turns one mistake into a wrong
// answer instead of a missing one.
func (m *dcMachine) execute(source string) error {
	for index := 0; index < len(source) && !m.quit; index++ {
		character := source[index]
		if character == ' ' || character == '\t' || character == '\n' || character == '\r' {
			continue
		}
		used, err := m.step(source, index)
		if err != nil {
			return err
		}
		index += used
	}
	return nil
}
