package main

import (
	"fmt"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/version"
)

// infoFlag answers a question about the binary itself and exits, without
// reading a script, entering the REPL, or touching the terminal. Both spellings
// have to work on a machine where stdin is not a TTY, because a package manager
// asks them during install.
//
// Reported as (handled, error) rather than by returning a sentinel, so the
// caller keeps its ordinary dispatch when the argument is not one of these.
func (c command) infoFlag(argument string) (bool, error) {
	switch argument {
	case "--version":
		fmt.Fprintln(c.stdout, version.Describe())
		return true, nil
	case "--list":
		// One name per line, sorted, no decoration: this output generates the
		// Scoop shims, so anything else on the line becomes a broken shim.
		for _, name := range applets.DefaultRegistry.Names() {
			fmt.Fprintln(c.stdout, name)
		}
		return true, nil
	case "--help":
		// Help was asked for, so it is output: stdout and status 0. A
		// diagnostic is the other thing, and goes to stderr with a non-zero
		// status. Conflating them makes `nemosh --help | less` show nothing.
		fmt.Fprint(c.stdout, helpText())
		return true, nil
	}
	return false, nil
}

// helpText describes the binary that exists rather than a shell in general. The
// applet count is read from the registry, so adding one cannot leave this
// quietly wrong.
func helpText() string {
	return fmt.Sprintf(`nemosh - a Windows-first, BusyBox-style POSIX shell and utility bundle

Usage: nemosh [-c COMMAND [NAME [ARG]...]]
   or: nemosh [-i]
   or: nemosh SCRIPT [ARG]...
   or: nemosh -
   or: nemosh APPLET [ARG]...
   or: APPLET [ARG]...          when invoked under an applet's name

Options:
  -c COMMAND    run COMMAND and exit; NAME becomes $0 and ARG... the positionals
  -i            run interactively even when stdin is not a terminal
  -             read the script from standard input
  --version     print the version and exit
  --list        print every bundled applet, one per line, and exit
  --help        print this and exit

Nemosh is a multicall binary: invoked under an applet's name, directly or
through a shim, it runs that applet instead of the shell. %d applets are
bundled; "nemosh --list" names them.

Applets carry no usage text. An option one does not implement is refused by
name rather than ignored, so a script asking for it fails instead of quietly
getting something else. docs/support-matrix.md records what each applet
accepts, measured against a built binary rather than claimed.

Environment:
  NEMOSH_DEBUG=path,exec,fd     trace resolution on the named channels
  NEMOSH_OVERRIDE_APPLETS       names the shell resolves as external commands
                                rather than applets, separated by spaces,
                                commas, or semicolons. Scoped to shell lookup:
                                "nemosh cat" and a cat shim stay unconditional

Home page: https://github.com/xiongnemo/nemosh
`, len(applets.DefaultRegistry.Names()))
}
