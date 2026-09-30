package runtime

import (
	"errors"
	"runtime"
	"strings"
)

// unixPathCommand is the command a name in /bin, /usr/bin, /sbin or /usr/sbin stands for on
// Windows when there is no such file, as busybox-w32's find_command reads one (shell/ash.c:9784):
// its last part, run as the builtin, the applet or the program on PATH of that name. So
// `/bin/echo hi`, `/usr/bin/true` and `/bin/cat file`, written for a Unix machine, run where they
// were not found. A function is passed over, as busybox passes it over, unless the name is a
// builtin and no applet: `/bin/echo` is never an `echo` function. A file that is there runs as
// written, and so does every name on Linux and macOS, where busybox has no such rule.
func (r Runtime) unixPathCommand(args []string, allowFunctions bool) ([]string, bool) {
	if runtime.GOOS != "windows" || !unixInterpreterPath(args[0]) {
		return args, allowFunctions
	}
	name := args[0][strings.LastIndex(args[0], "/")+1:]
	if _, err := r.externalCommandPath(args[0]); name == "" || !errors.Is(err, errExternalNotFound) {
		return args, allowFunctions
	}
	_, applet := r.lookupApplet(name)
	return append([]string{name}, args[1:]...), allowFunctions && isRuntimeBuiltin(name) && !applet
}
