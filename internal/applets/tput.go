package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// tput, ncurses', speaking xterm-256color's terminfo: the VT dialect the Windows console and
// Windows Terminal understand, whatever TERM says, there being no terminfo database here to look
// another up in. String capabilities are written as terminfo has them, without a newline; numbers
// with one. cols and lines are COLUMNS and LINES when set, as ncurses lets the environment win,
// then the terminal's, from standard output or standard error, then 80 and 24. An unknown
// capability is status 4, as ncurses' tput has it. busybox-w32 has no tput.

// tputStrings are xterm-256color's string capabilities that take no parameter.
func tputStrings(name string) (string, bool) {
	switch name {
	case "bold":
		return "\x1b[1m", true
	case "dim":
		return "\x1b[2m", true
	case "sitm":
		return "\x1b[3m", true
	case "ritm":
		return "\x1b[23m", true
	case "smul":
		return "\x1b[4m", true
	case "rmul":
		return "\x1b[24m", true
	case "blink":
		return "\x1b[5m", true
	case "rev", "smso":
		return "\x1b[7m", true
	case "rmso":
		return "\x1b[27m", true
	case "sgr0":
		return "\x1b(B\x1b[m", true
	case "clear":
		return "\x1b[H\x1b[2J", true
	case "el":
		return "\x1b[K", true
	case "el1":
		return "\x1b[1K", true
	case "ed":
		return "\x1b[J", true
	case "home":
		return "\x1b[H", true
	case "civis":
		return "\x1b[?25l", true
	case "cnorm":
		return "\x1b[?12l\x1b[?25h", true
	case "smcup":
		return "\x1b[?1049h\x1b[22;0;0t", true
	case "rmcup":
		return "\x1b[?1049l\x1b[23;0;0t", true
	case "sc":
		return "\x1b7", true
	case "rc":
		return "\x1b8", true
	case "cuu1":
		return "\x1b[A", true
	case "cud1":
		return "\n", true
	case "cuf1":
		return "\x1b[C", true
	case "cub1":
		return "\b", true
	case "bel":
		return "\a", true
	case "longname":
		return "xterm with 256 colors", true
	}
	return "", false
}

func newTputApplet() Applet {
	return simpleApplet{name: "tput", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) error {
		options, operands, err := parseAppletOptions(ctx, args, "", "T")
		if err != nil {
			return err
		}
		if term := options.value('T'); options.has('T') && term != "xterm-256color" && term != "xterm" {
			return ExitStatusMessage(3, fmt.Errorf("unknown terminal \"%s\": this tput speaks xterm-256color alone", term))
		}
		if len(operands) == 0 {
			return ExitStatusMessage(2, errors.New("usage: tput [-T TERM] CAPABILITY [PARAMETER]..."))
		}
		name, parameters := operands[0], operands[1:]
		number := func(index int) int {
			if index >= len(parameters) {
				return 0
			}
			value, _ := strconv.Atoi(parameters[index])
			return value
		}
		view := ProcessViewFromContext(ctx)
		var answer string
		switch name {
		case "cols":
			answer = strconv.Itoa(tputSize(view, "COLUMNS", stdout, stderr, true, 80)) + "\n"
		case "lines":
			answer = strconv.Itoa(tputSize(view, "LINES", stdout, stderr, false, 24)) + "\n"
		case "colors":
			answer = "256\n"
		case "setaf", "setab":
			answer = tputColour(name == "setab", number(0))
		case "cup":
			answer = fmt.Sprintf("\x1b[%d;%dH", number(0)+1, number(1)+1)
		case "hpa":
			answer = fmt.Sprintf("\x1b[%dG", number(0)+1)
		case "vpa":
			answer = fmt.Sprintf("\x1b[%dd", number(0)+1)
		default:
			var known bool
			if answer, known = tputStrings(name); !known {
				return ExitStatusMessage(4, fmt.Errorf("unknown terminfo capability '%s'", name))
			}
		}
		_, err = io.WriteString(stdout, answer)
		return err
	}}
}

// tputColour is setaf's or setab's: 30 to 37 for the first eight, 90 to 97 for the next, and
// 38;5;N past them, 40, 100 and 48 behind, as xterm-256color's terminfo computes them.
func tputColour(background bool, colour int) string {
	base, bright, extended := 30, 90, 38
	if background {
		base, bright, extended = 40, 100, 48
	}
	switch {
	case colour < 8:
		return fmt.Sprintf("\x1b[%dm", base+colour)
	case colour < 16:
		return fmt.Sprintf("\x1b[%dm", bright+colour-8)
	}
	return fmt.Sprintf("\x1b[%d;5;%dm", extended, colour)
}

// tputSize is the environment's value for it, or the terminal's width or height, or fallback.
func tputSize(view ProcessView, variable string, stdout, stderr io.Writer, width bool, fallback int) int {
	if value, ok := view.LookupEnv(variable); ok {
		if size, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && size > 0 {
			return size
		}
	}
	for _, stream := range []io.Writer{stdout, stderr} {
		if file := stdoutFile(stream); file != nil {
			if columns, rows, err := term.GetSize(int(file.Fd())); err == nil && columns > 0 {
				if width {
					return columns
				}
				return rows
			}
		}
	}
	return fallback
}
