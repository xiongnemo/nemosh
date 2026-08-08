package applets

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// The escapes busybox-w32 emits, measured on 2026-08-09: a directory is
// \033[1;34m, an executable \033[1;32m, anything else \033[0;0m, and every name
// is closed with \033[m rather than the fuller \033[0m.
const (
	lsColorDirectory  = "\033[1;34m"
	lsColorExecutable = "\033[1;32m"
	lsColorPlain      = "\033[0;0m"
	lsColorReset      = "\033[m"
)

// colorWhen is the argument to --color.
type colorWhen int

const (
	colorNever colorWhen = iota
	colorAlways
	colorAuto
)

// parseColorWhen reads the value of --color. GNU's spellings, which busybox
// takes too: an absent value means always.
func parseColorWhen(value string, present bool) (colorWhen, error) {
	if !present {
		return colorAlways, nil
	}
	switch value {
	case "always", "force", "yes":
		return colorAlways, nil
	case "never", "none", "no":
		return colorNever, nil
	case "auto", "tty", "if-tty":
		return colorAuto, nil
	}
	return colorNever, fmt.Errorf("unsupported --color value: %s; this build implements always, never and auto", value)
}

// colorEnabled resolves auto against the stream actually being written to.
//
// Resolving it here rather than at parse time is what makes
// `alias ls='ls --color=auto'` safe to pipe: the same command colours a
// terminal and stays plain into `grep`.
func colorEnabled(when colorWhen, stdout io.Writer) bool {
	switch when {
	case colorAlways:
		return true
	case colorAuto:
		file, ok := stdout.(*os.File)
		return ok && term.IsTerminal(int(file.Fd()))
	}
	return false
}

// colorForEntry picks the escape for one name.
func colorForEntry(info os.FileInfo) string {
	if info == nil {
		return lsColorPlain
	}
	if info.IsDir() {
		return lsColorDirectory
	}
	if isExecutableEntry(info) {
		return lsColorExecutable
	}
	return lsColorPlain
}

// isExecutableEntry asks the question the platform can answer. Windows has no
// execute bit, so busybox decides from the suffix (win32/mingw.c), and the same
// list the shell uses for lookup is the honest one to reuse.
func isExecutableEntry(info os.FileInfo) bool {
	if info.Mode().Perm()&0o111 != 0 {
		return true
	}
	name := strings.ToLower(info.Name())
	for _, suffix := range [...]string{".com", ".exe", ".bat", ".cmd"} {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

// paintLsName wraps a name in its colour, or returns it unchanged.
func paintLsName(name string, info os.FileInfo, colored bool) string {
	if !colored {
		return name
	}
	return colorForEntry(info) + name + lsColorReset
}
