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
// would use, in the spelling the shell uses.
func (r Runtime) whichBuiltin(args []string) int {
	if len(args) == 0 {
		// busybox exits 1 with no output, which is also what `which` on a name
		// it cannot find does -- there is nothing to report either way.
		return 1
	}
	status := 0
	for _, name := range args {
		// -a would list every match; this reports the one that would run, so an
		// answer is never a list the reader has to choose from.
		if r.isKnownCommand(name) {
			fmt.Fprintln(r.streams.Stdout, name)
			continue
		}
		resolved, err := r.externalCommandPath(name)
		if err != nil {
			// Silent, as which is: the exit status is the answer, and a script
			// testing `which foo >/dev/null` wants no noise on the way.
			status = 1
			continue
		}
		fmt.Fprintln(r.streams.Stdout, filepath.ToSlash(resolved))
	}
	return status
}
