package main

import (
	"fmt"
	"os"
	"os/user"
	"strings"
	"unicode/utf8"

	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

const (
	defaultPS1 = `# \u @ \h in \w\n\$ `
	defaultPS2 = `> `
)

type promptValues struct {
	username         string
	hostname         string
	workingDirectory string
	symbol           string
}

func interactivePrompt(rt runtime.Runtime, continuation bool) string {
	name, fallback := "PS1", defaultPS1
	if continuation {
		name, fallback = "PS2", defaultPS2
	}
	value, present := rt.LookupVariable(name)
	if !present {
		value = fallback
	}
	return renderPrompt(value, currentPromptValues(rt))
}

func currentPromptValues(rt runtime.Runtime) promptValues {
	return promptValues{
		username:         promptUsername(rt),
		hostname:         promptHostname(),
		workingDirectory: rt.WorkingDirectory(),
		symbol:           promptSymbol(),
	}
}

func promptUsername(rt runtime.Runtime) string {
	for _, name := range []string{"USER", "USERNAME"} {
		if value, present := rt.LookupVariable(name); present && value != "" {
			return value
		}
	}
	current, err := user.Current()
	if err != nil {
		return "user"
	}
	if _, name, found := strings.Cut(current.Username, `\`); found {
		return name
	}
	return current.Username
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
		default:
			rendered.WriteByte('\\')
			rendered.WriteByte(value[index])
		}
	}
	return rendered.String()
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
