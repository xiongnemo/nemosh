package applets

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// The rest of the everyday commands a clean Windows machine does not have.
//
// Measured: of the names on this list, Windows itself ships only `where`,
// `whoami` and `timeout` -- and the last of those is a trap, since Windows'
// `timeout.exe` is a *sleep*. `timeout 5 cmd` there waits five seconds and never
// runs cmd, silently, exit 0. Owning the name is the only way to stop a script
// getting nothing instead of a failure.

// tee copies its input to stdout and to each named file, which is what a
// pipeline needs to keep a copy of something it is also passing on.
func newTeeApplet() Applet {
	return simpleApplet{name: "tee", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, paths, err := parseAppletOptions(args, "a", "")
		if err != nil {
			return err
		}
		writers := []io.Writer{stdout}
		view := ProcessViewFromContext(ctx)
		for _, path := range paths {
			native, err := resolveHostPath(view, path)
			if err != nil {
				return err
			}
			flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
			if options.has('a') {
				flags = os.O_WRONLY | os.O_CREATE | os.O_APPEND
			}
			file, err := os.OpenFile(native, flags, 0o644)
			if err != nil {
				return cannotOpen(path, err)
			}
			defer file.Close()
			writers = append(writers, file)
		}
		_, err = copyWithContext(ctx, io.MultiWriter(writers...), stdin)
		return err
	}}
}

// seq counts, which is what a loop over a range needs and what `for i in 1 2 3`
// stops being able to express at about ten.
//
// The three forms are busybox's: LAST, FIRST LAST, and FIRST INCREMENT LAST.
func newSeqApplet() Applet {
	return simpleApplet{name: "seq", run: func(args []string, _ io.Reader, stdout, _ io.Writer) error {
		numbers := make([]int, 0, 3)
		for _, arg := range args {
			value, err := strconv.Atoi(arg)
			if err != nil {
				return fmt.Errorf("invalid number: %s", arg)
			}
			numbers = append(numbers, value)
		}
		first, increment, last := 1, 1, 0
		switch len(numbers) {
		case 1:
			last = numbers[0]
		case 2:
			first, last = numbers[0], numbers[1]
		case 3:
			first, increment, last = numbers[0], numbers[1], numbers[2]
		default:
			return missingOperand()
		}
		if increment == 0 {
			return fmt.Errorf("invalid increment: 0")
		}
		for value := first; (increment > 0 && value <= last) || (increment < 0 && value >= last); value += increment {
			if _, err := fmt.Fprintln(stdout, value); err != nil {
				return err
			}
		}
		return nil
	}}
}

// clear empties the screen. Ctrl-L already did this in the line editor; the
// command is what a script -- or an alias, which is how most people reach it --
// can call.
//
// The sequence is the one the editor emits: home, then erase the display.
func newClearApplet() Applet {
	return simpleApplet{name: "clear", run: func(args []string, _ io.Reader, stdout, _ io.Writer) error {
		if _, operands, err := parseAppletOptions(args, "", ""); err != nil {
			return err
		} else if len(operands) > 0 {
			return fmt.Errorf("extra operand '%s'", operands[0])
		}
		_, err := io.WriteString(stdout, "\033[H\033[2J")
		return err
	}}
}

// whoami is the name that belongs to this process's identity, which under
// elevation is root.
//
// Windows ships whoami.exe and it answers differently: `DOMAIN\user`, where this
// says `user` -- or `root`, which Windows has no notion of at all. A script
// comparing the output needs the busybox answer, and inside this shell an applet
// is what it gets.
func newWhoamiApplet() Applet {
	return simpleApplet{name: "whoami", run: func(args []string, _ io.Reader, stdout, _ io.Writer) error {
		if _, operands, err := parseAppletOptions(args, "", ""); err != nil {
			return err
		} else if len(operands) > 0 {
			return fmt.Errorf("extra operand '%s'", operands[0])
		}
		_, err := fmt.Fprintln(stdout, currentIdentity().user)
		return err
	}}
}

// mktemp makes a file nobody else has, which is the only safe way for a script
// to need a scratch file.
//
// The template's trailing Xs are replaced, as everywhere else, and -d makes a
// directory instead. Creation is what guarantees uniqueness: a name that is
// merely unused when it is chosen is a race, and os.CreateTemp closes it.
func newMktempApplet() Applet {
	return simpleApplet{name: "mktemp", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
		options, operands, err := parseAppletOptions(args, "dqu", "")
		if err != nil {
			return err
		}
		template := "tmp.XXXXXX"
		if len(operands) > 0 {
			template = operands[0]
		}
		if len(operands) > 1 {
			return fmt.Errorf("extra operand '%s'", operands[1])
		}
		directory, pattern := filepath.Split(filepath.FromSlash(template))
		if directory == "" {
			directory = os.TempDir()
		} else {
			view := ProcessViewFromContext(ctx)
			if directory, err = resolveHostPath(view, directory); err != nil {
				return err
			}
		}
		// Go's CreateTemp puts its random part where a `*` is, and appends when
		// there is none; the X form is what every script writes.
		pattern = strings.Replace(strings.TrimRight(pattern, "X"), "*", "", -1) + "*"
		created, err := makeTemporary(directory, pattern, options.has('d'))
		if err != nil {
			return cannotCreate(template, err)
		}
		_, err = fmt.Fprintln(stdout, filepath.ToSlash(created))
		return err
	}}
}

func makeTemporary(directory, pattern string, wantDirectory bool) (string, error) {
	if wantDirectory {
		return os.MkdirTemp(directory, pattern)
	}
	file, err := os.CreateTemp(directory, pattern)
	if err != nil {
		return "", err
	}
	name := file.Name()
	return name, file.Close()
}
