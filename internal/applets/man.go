package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/xiongnemo/nemosh/internal/capability"
)

// man, apropos, whatis and info: this shell's own help, as man-db would put it. There are no man
// pages on Windows to format, and busybox-w32's man, which formats them, has none to read; what
// every command here takes is its help, so `man ls` is `help ls`, paged by less on a terminal. A
// SECTION before the NAME answers for 1 and 8, the commands; man -k and apropos search the names
// and the summaries by regular expression, as apropos does, man -f and whatis match a name, each
// in man-db's `NAME (1)               - description`. A page that is not here is man-db's words and
// status 16. info shows the same, as GNU info falls back to a man page where there is no Info
// manual.

// manApplet is a named type, as env's is, so its Run, which asks commandFor for less, is not part
// of the registry's initialization: a closure would make the registry refer to itself.
type manApplet struct{ name string }

func newManApplet(name string) Applet { return manApplet{name: name} }

func (m manApplet) Name() string { return m.name }

func (m manApplet) Run(ctx context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) error {
	name := m.name
	flags := ""
	if name == "man" {
		flags = "kf"
	}
	options, operands, err := parseAppletOptions(ctx, args, flags, "")
	if err != nil {
		return err
	}
	switch {
	case name == "apropos" || options.has('k'):
		return writeManSearch(stdout, operands, false)
	case name == "whatis" || options.has('f'):
		return writeManSearch(stdout, operands, true)
	case len(operands) == 0:
		return errors.New("What manual page do you want?\nFor example, try 'man man'.")
	}
	section := ""
	if len(operands) > 1 && operands[0] != "" && operands[0][0] >= '0' && operands[0][0] <= '9' {
		section, operands = operands[0], operands[1:]
	}
	var pages strings.Builder
	for _, page := range operands {
		text, found := capability.UsageFor(page)
		switch {
		case section != "" && section != "1" && section != "8":
			return ExitStatusMessage(16, fmt.Errorf("No manual entry for %s in section %s", page, section))
		case !found:
			return ExitStatusMessage(16, fmt.Errorf("No manual entry for %s", page))
		}
		pages.WriteString(text)
	}
	if stdoutIsTerminal(stdout) {
		if less, ok := commandFor(ctx, "less"); ok {
			return less.Run(ctx, nil, strings.NewReader(pages.String()), stdout, stderr)
		}
	}
	_, err = io.WriteString(stdout, pages.String())
	return err
}

// writeManSearch lists the commands each keyword finds: whose name it is, with whatis, or whose
// name or summary its regular expression matches, without regard to case, with apropos.
func writeManSearch(stdout io.Writer, keywords []string, exact bool) error {
	if len(keywords) == 0 {
		return errors.New("what?")
	}
	var lines strings.Builder
	var missing []string
	for _, keyword := range keywords {
		pattern, err := regexp.Compile("(?i)" + keyword)
		if err != nil && !exact {
			return ExitStatusMessage(2, fmt.Errorf("%s: %v", keyword, err))
		}
		found := false
		for _, summary := range capability.Summaries() {
			if exact && summary.Name == keyword || !exact && (pattern.MatchString(summary.Name) || pattern.MatchString(summary.Line)) {
				fmt.Fprintf(&lines, "%-20s - %s\n", summary.Name+" (1)", summary.Line)
				found = true
			}
		}
		if !found {
			missing = append(missing, keyword)
		}
	}
	if _, err := io.WriteString(stdout, lines.String()); err != nil {
		return err
	}
	if len(missing) > 0 {
		return ExitStatusMessage(16, fmt.Errorf("%s: nothing appropriate.", strings.Join(missing, ", ")))
	}
	return nil
}
