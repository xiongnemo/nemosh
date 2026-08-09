package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// The default prompt carries colour, because a prompt that does not is the
// first thing a user replaces. Only the eight original foreground colours and
// bold are used: those survive every terminal Nemosh runs on, including a plain
// conhost, where the 256-colour and truecolour forms do not.
//
// Every sequence is closed with a reset, so a prompt cannot leave the terminal
// tinted for the command that follows.
const (
	promptReset  = "\033[0m"
	promptBlue   = "\033[1;34m"
	promptGreen  = "\033[1;32m"
	promptYellow = "\033[0;33m"
	promptRed    = "\033[1;31m"
	promptDim    = "\033[2m"
)

var (
	defaultPS1 = promptBlue + `# \u` + promptReset + promptDim + ` @ ` + promptReset +
		promptGreen + `\h` + promptReset + promptDim + ` in ` + promptReset +
		promptYellow + `\w` + promptReset + "\n" + promptRed + `\$` + promptReset + ` `
	defaultPS2 = promptDim + `>` + promptReset + ` `
)

type promptValues struct {
	username         string
	hostname         string
	workingDirectory string
	symbol           string
}

func interactivePrompt(rt runtime.Runtime, continuation bool) string {
	return interactivePromptWithStatus(context.Background(), rt, continuation, 0)
}

// The prompt is expanded before its backslash escapes are rendered. That order
// is deliberate: rendering first would feed a directory name back into the
// parser, so a directory called `$(...)` would run it.
func interactivePromptWithStatus(ctx context.Context, rt runtime.Runtime, continuation bool, lastStatus int) string {
	name, fallback := "PS1", defaultPS1
	if continuation {
		name, fallback = "PS2", defaultPS2
	}
	value, present := rt.LookupVariable(name)
	if !present {
		value = fallback
	}
	return renderPrompt(rt.ExpandPromptString(ctx, value, lastStatus), currentPromptValues(rt))
}

func currentPromptValues(rt runtime.Runtime) promptValues {
	return promptValues{
		username:         promptUsername(rt),
		hostname:         promptHostname(),
		workingDirectory: rt.WorkingDirectory(),
		symbol:           promptSymbol(),
	}
}

// promptUsername is `\u`, and it follows the shell's own identity rather than
// the environment.
//
// bash and busybox both take `\u` from the passwd entry for the effective uid,
// not from $USER. Reading the variable first is what made an elevated nemosh
// disagree with itself: under gsudo `id -u` answered 0 and the prompt still read
// `nemo`, because Windows leaves USERNAME alone when a process is elevated.
// busybox says `root` there, and now so does this.
//
// The variables remain a fallback for the case the identity cannot be
// determined at all, which is the only thing they were really covering.
func promptUsername(rt runtime.Runtime) string {
	if name := applets.CurrentUserName(); name != "" {
		return name
	}
	for _, name := range []string{"USER", "USERNAME"} {
		if value, present := rt.LookupVariable(name); present && value != "" {
			return value
		}
	}
	return "user"
}

func promptHostname() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		return "host"
	}
	short, _, _ := strings.Cut(hostname, ".")
	return short
}

func renderPrompt(value string, values promptValues) string {
	var rendered strings.Builder
	for index := 0; index < len(value); index++ {
		if value[index] != '\\' || index+1 == len(value) {
			rendered.WriteByte(value[index])
			continue
		}
		index++
		switch value[index] {
		case 'u':
			appendPromptValue(&rendered, values.username)
		case 'h':
			appendPromptValue(&rendered, values.hostname)
		case 'w':
			appendPromptValue(&rendered, values.workingDirectory)
		case 'n':
			rendered.WriteByte('\n')
		case '$':
			appendPromptValue(&rendered, values.symbol)
		case '\\':
			rendered.WriteByte('\\')
		case 'e':
			rendered.WriteByte(0x1b)
		case 'a':
			rendered.WriteByte(0x07)
		case 'r':
			rendered.WriteByte('\r')
		case '[', ']':
			// bash's markers around a non-printing run. Nemosh does not measure
			// the prompt, so they carry no information here and are dropped
			// rather than printed as literal brackets.
		case '0', '1', '2', '3', '4', '5', '6', '7':
			// An octal escape, which is how a startup file written for ash or
			// bash spells a colour: `\033[1;34m`. Up to three digits, which is
			// what bash and printf both read.
			octal, consumed := readOctalEscape(value, index)
			rendered.WriteByte(octal)
			index += consumed - 1
		default:
			rendered.WriteByte('\\')
			rendered.WriteByte(value[index])
		}
	}
	return rendered.String()
}

// readOctalEscape reads up to three octal digits starting at index, returning
// the byte they denote and how many digits were consumed.
func readOctalEscape(text string, index int) (byte, int) {
	value, consumed := 0, 0
	for consumed < 3 && index+consumed < len(text) {
		digit := text[index+consumed]
		if digit < '0' || digit > '7' {
			break
		}
		value = value*8 + int(digit-'0')
		consumed++
	}
	return byte(value), consumed
}

func appendPromptValue(rendered *strings.Builder, value string) {
	for len(value) > 0 {
		character, width := utf8.DecodeRuneInString(value)
		if character == utf8.RuneError && width == 1 {
			fmt.Fprintf(rendered, `\x%02x`, value[0])
			value = value[1:]
			continue
		}
		if character < ' ' || character == 0x7f || (0x80 <= character && character <= 0x9f) {
			fmt.Fprintf(rendered, `\x%02x`, character)
		} else {
			rendered.WriteString(value[:width])
		}
		value = value[width:]
	}
}
