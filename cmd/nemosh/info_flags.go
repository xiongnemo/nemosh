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
	}
	return false, nil
}
