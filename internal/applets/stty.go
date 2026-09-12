package applets

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// stty, scoped to what a Windows console can actually answer.
//
// The full command sets forty-odd termios flags, most of which describe a serial line: parity,
// baud rate, flow control, the erase and kill characters. **Windows has none of that.** Its
// console is a different object with a handful of mode bits, and the honest set of questions
// it can answer is small:
//
//	stty size        the window, as ROWS COLS
//	stty -echo       stop echoing what is typed -- the reason scripts call stty at all
//	stty echo        start again
//	stty sane        put it back the way a shell expects it
//	stty -a          report what this can say
//
// Everything else is **refused by name**. A `stty` that accepted `-icanon min 1 time 0` and
// did nothing would be worse than one that is not there: the script would carry on believing
// the terminal had changed. busybox-w32's own stty reports `Bad file descriptor` for most of
// its options on a Windows console, which is the same conclusion reached less clearly.
//
// The settings named here are the ones `read -s` and a password prompt need, which is what
// nearly every real use of stty in a script comes down to.
func newSttyApplet() Applet {
	return simpleApplet{name: "stty", run: func(args []string, _ io.Reader, stdout, _ io.Writer) error {
		_, operands, err := parseAppletOptions(args, "a", "")
		if err != nil {
			// -a is the one dashed word getopt should take; the rest are settings and
			// are read below, so a parse failure here is a real one.
			return err
		}
		settings := sttySettings(args)
		handle, ok := sttyTerminal()
		if !ok {
			return fmt.Errorf("standard input: not a terminal")
		}
		if len(settings) == 0 || (len(settings) == 1 && settings[0] == "-a") {
			return writeSttyReport(stdout, handle, len(settings) == 1)
		}
		_ = operands
		return applySttySettings(stdout, handle, settings)
	}}
}

// sttySettings keeps the arguments in the order they were written, because stty's are
// positional -- `stty -echo echo` ends with echo on.
func sttySettings(args []string) []string {
	settings := make([]string, 0, len(args))
	settings = append(settings, args...)
	return settings
}

// sttyTerminal answers the handle to ask about, preferring standard input.
//
// Standard input first because that is what stty describes, and standard output second
// because `stty size > file` is a thing people write and the window is the same either way.
func sttyTerminal() (*os.File, bool) {
	for _, file := range []*os.File{os.Stdin, os.Stdout, os.Stderr} {
		if term.IsTerminal(int(file.Fd())) {
			return file, true
		}
	}
	return nil, false
}

func writeSttyReport(out io.Writer, handle *os.File, verbose bool) error {
	columns, rows, err := term.GetSize(int(handle.Fd()))
	if err != nil {
		return fmt.Errorf("cannot read the terminal size: %s", CauseText(err))
	}
	echo, err := terminalEcho(handle)
	if err != nil {
		return err
	}
	state := "echo"
	if !echo {
		state = "-echo"
	}
	if !verbose {
		// The bare form is one line, as the references' is -- theirs lists speed and a
		// line discipline, neither of which a Windows console has.
		_, err := fmt.Fprintf(out, "rows %d; columns %d; %s\n", rows, columns, state)
		return err
	}
	_, err = fmt.Fprintf(out, "rows %d; columns %d;\n%s\n", rows, columns, state)
	return err
}

func applySttySettings(out io.Writer, handle *os.File, settings []string) error {
	for _, setting := range settings {
		switch setting {
		case "size":
			columns, rows, err := term.GetSize(int(handle.Fd()))
			if err != nil {
				return fmt.Errorf("cannot read the terminal size: %s", CauseText(err))
			}
			if _, err := fmt.Fprintf(out, "%d %d\n", rows, columns); err != nil {
				return err
			}
		case "echo":
			if err := setTerminalEcho(handle, true); err != nil {
				return err
			}
		case "-echo":
			if err := setTerminalEcho(handle, false); err != nil {
				return err
			}
		case "sane", "cooked":
			// What a shell expects to find: echo on, and whatever else the platform
			// layer considers normal.
			if err := makeTerminalSane(handle); err != nil {
				return err
			}
		default:
			return fmt.Errorf("invalid argument '%s': this stty knows size, echo, -echo, sane and -a", strings.TrimSpace(setting))
		}
	}
	return nil
}
