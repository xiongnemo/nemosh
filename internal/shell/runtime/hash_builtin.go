package runtime

import (
	"fmt"
	"strings"
)

// hashBuiltin is busybox's `hash [-r] [name ...]` without the table. Each name is looked up
// as a command would be, and one that cannot be found is "not found", status 1: that is what
// a script asks it, `hash git 2>/dev/null || die "git is needed"`. It was refused. Lookup here
// is never cached, so there is nothing to remember: `hash` lists nothing and `hash -r` forgets
// nothing, where busybox lists and forgets the paths it has remembered. As in busybox, a
// function, a builtin or an applet is found, a name with a slash is passed over, and names
// after -r are not looked up.
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
		return 0
	}
	status := 0
	for _, name := range args[index:] {
		if strings.ContainsRune(name, '/') || r.isKnownCommand(name) {
			continue
		}
		if _, err := r.externalCommandPath(name); err != nil {
			fmt.Fprintf(r.streams.Stderr, "%shash: %s: not found\n", r.diagnosticPrefix(), name)
			status = 1
		}
	}
	return status
}
