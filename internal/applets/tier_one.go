package applets

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
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
// The template's trailing X run is replaced by that many random characters, and
// -d makes a directory instead. Creation is what guarantees uniqueness: a name
// that is merely unused when it is chosen is a race. The rules about where the
// name lands and what a template must look like are in mktemp.go, along with what
// they were before.
func newMktempApplet() Applet {
	return simpleApplet{name: "mktemp", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
		options, operands, err := parseAppletOptions(args, "dqut", "p")
		if err != nil {
			return err
		}
		if len(operands) > 1 {
			return fmt.Errorf("extra operand '%s'", operands[1])
		}
		created, err := runMktemp(ctx, options, operands)
		if err != nil {
			// -q asks for the status without the diagnostic, which is what a script
			// testing `mktemp ... || fallback` wants.
			if options.has('q') {
				return ErrExitFalse
			}
			return err
		}
		_, err = fmt.Fprintln(stdout, filepath.ToSlash(created))
		return err
	}}
}

// runMktemp resolves where the name goes and makes it. See mktemp.go for the four rules this
// gets right that the previous one did not.
func runMktemp(ctx context.Context, options appletOptions, operands []string) (string, error) {
	template := "tmp.XXXXXX"
	if len(operands) > 0 {
		template = operands[0]
	}
	spec, err := parseTemporaryTemplate(template)
	if err != nil {
		return "", err
	}
	// -p names the base directory and busybox has it imply -t; -t alone means $TMPDIR. A
	// template with no directory part of its own is relative to the current directory,
	// which is the rule both references apply and the one this applet did not.
	base := ""
	switch {
	case options.has('p'):
		base = options.values['p']
	case options.has('t') || len(operands) == 0:
		base = temporaryDirectory(ctx)
	}
	if base != "" {
		if spec.directory != "" {
			// `mktemp -t a/bXXXXXX` cannot mean anything: the base directory and the
			// template's own directory are two answers to one question. uutils words
			// the refusal this way and busybox ignores the template's directory
			// silently, which is worse.
			return "", fmt.Errorf("invalid template, '%s', contains directory separator", template)
		}
		spec.directory = base
	}
	// Always resolved, including the empty case as ".": a relative name has to be created
	// where the *shell* is, and `cd` here does not move the process. See mktemp.go.
	source := spec.directory
	if source == "" {
		source = "."
	}
	if spec.hostDirectory, err = resolveHostPath(ProcessViewFromContext(ctx), source); err != nil {
		return "", err
	}
	created, err := createTemporary(spec, options.has('d'), options.has('u'))
	if err != nil {
		return "", cannotCreate(template, err)
	}
	return created, nil
}

// temporaryDirectory is $TMPDIR as the shell sees it, falling back to the host's. The
// environment is consulted through the process view so a script that exported TMPDIR gets the
// directory it asked for.
func temporaryDirectory(ctx context.Context) string {
	if view := ProcessViewFromContext(ctx); view != nil {
		if value, ok := view.LookupEnv("TMPDIR"); ok && value != "" {
			return value
		}
	}
	return os.TempDir()
}
