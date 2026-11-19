package runtime

import (
	"fmt"
	"path/filepath"
)

// which is `command -v` under the name everyone actually types.
//
// It is a builtin rather than an applet, and that is the point: the lookup has
// to be the shell's own, so the answer cannot disagree with what typing the name
// would run. An applet would have had to re-implement PATH search and the
// Windows suffix order beside the copy in external.go, and two copies of a
// lookup drift -- which is precisely the defect `command -v` was fixed for, when
// it said nothing about `git` while plain `git` ran.
//
// busybox's which is an applet because there the lookup is libc's and shared.
// Here it is not, so the name follows the knowledge.
//
// Windows has `where.exe`, which is neither the same name nor the same output:
// it prints every match and speaks in backslashes. This prints the one the shell
// would use, in the spelling the shell uses, and with -a, busybox's and Debian's
// option, every other after it.
func (r Runtime) whichBuiltin(args []string) int {
	all, names, ok := r.whichOptions(args)
	if !ok {
		return 1
	}
	if len(names) == 0 {
		// busybox exits 1 with no output, which is also what `which` on a name
		// it cannot find does -- there is nothing to report either way.
		return 1
	}
	status := 0
	for _, name := range names {
		found := false
		if r.isKnownCommand(name) {
			fmt.Fprintln(r.streams.Stdout, name)
			if !all {
				continue
			}
			found = true
		}
		// Same reason as in commandV: dispatch refuses these before PATH, so a
		// file of that name on PATH must not be offered as the answer.
		if isUnimplementedBuiltin(name) {
			status = 1
			continue
		}
		var paths []string
		if all {
			paths = r.everyCommandPath(name)
		} else if resolved, err := r.knownCommandPath(name); err == nil {
			paths = []string{resolved}
		}
		for _, resolved := range paths {
			fmt.Fprintln(r.streams.Stdout, filepath.ToSlash(resolved))
		}
		// Silent when there is nothing, as which is: the exit status is the answer,
		// and a script testing `which foo >/dev/null` wants no noise on the way.
		if !found && len(paths) == 0 {
			status = 1
		}
	}
	return status
}

// whichOptions reads -a and --. Anything else is busybox's `unknown option`: it was a name to
// look for, so `which -a python` looked for a command called -a and said 1 with python on PATH.
func (r Runtime) whichOptions(args []string) (bool, []string, bool) {
	all := false
	for len(args) > 0 && len(args[0]) > 1 && args[0][0] == '-' {
		option := args[0]
		args = args[1:]
		if option == "--" {
			break
		}
		for _, letter := range option[1:] {
			if letter != 'a' {
				fmt.Fprintf(r.streams.Stderr, "%swhich: unknown option -- %c\n", r.diagnosticPrefix(), letter)
				return false, nil, false
			}
			all = true
		}
	}
	return all, args, true
}
