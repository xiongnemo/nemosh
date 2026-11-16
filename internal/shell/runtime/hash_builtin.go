package runtime

import (
	"fmt"
	"strings"
)

// hashBuiltin is busybox's hashcmd (shell/ash.c:11914): with no operand it lists where the
// commands that ran were found, a path a line, starred where a `cd` since asks for one to be
// checked again; -r forgets them all, and names after it are not looked up; a name is
// forgotten and looked up again, and one that cannot be found is "not found", status 1 --
// what a script asks it, `hash git 2>/dev/null || die "git is needed"`. As in busybox, a
// function, a builtin or an applet is found, and a name with a slash is passed over. See
// command_hash.go for the table, which until it was kept made `hash` list nothing and
// `hash -r` forget nothing.
func (r Runtime) hashBuiltin(args []string) int {
	index := 0
	for ; index < len(args); index++ {
		arg := args[index]
		if arg == "--" {
			index++
			break
		}
		if len(arg) < 2 || arg[0] != '-' {
			break
		}
		for _, letter := range arg[1:] {
			if letter != 'r' {
				fmt.Fprintf(r.streams.Stderr, "%shash: illegal option -%c\n", r.diagnosticPrefix(), letter)
				return 2
			}
		}
		r.commands.reset()
		return 0
	}
	if index == len(args) {
		for _, line := range r.commands.listing(r.vars["PATH"]) {
			fmt.Fprintln(r.streams.Stdout, line)
		}
		return 0
	}
	status := 0
	for _, name := range args[index:] {
		if strings.ContainsRune(name, '/') || r.isKnownCommand(name) {
			continue
		}
		r.commands.forget(r.vars["PATH"], name)
		if _, err := r.externalCommandPath(name); err != nil {
			fmt.Fprintf(r.streams.Stderr, "%shash: %s: not found\n", r.diagnosticPrefix(), name)
			status = 1
		}
	}
	return status
}
