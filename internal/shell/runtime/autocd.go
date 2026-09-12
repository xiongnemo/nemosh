package runtime

import (
	"errors"
	"os"
	"path/filepath"
)

// `shopt -s autocd`, and `CDPATH`.
//
// Two conveniences that sit either side of the same question -- "the word you typed is not
// a command; is it a directory?" -- and both are off unless asked for, because each one
// changes what a *mistake* does.
//
// autocd is the one people miss most coming from zsh: typing `src` goes there. It is
// checked only **after** command lookup has failed, so a command always wins. That
// ordering is the whole safety argument: many trees contain a directory called `test`, and
// `test` is a command.

// autoChangeDirectory handles a bare directory name when autocd is on.
//
// The second result says whether it took over, so the caller can report the ordinary
// not-found failure when it did not.
func (r Runtime) autoChangeDirectory(args []string, lookupErr error) (int, bool) {
	if !r.options.autoCD || len(args) != 1 || !errors.Is(lookupErr, errExternalNotFound) {
		return 0, false
	}
	// A word with no path separator is a directory only if it is one here. Anything
	// with a separator is already path-shaped, and `./x` or `../x` is the form people
	// type when they mean a place rather than a command.
	resolved, err := r.ResolveNemoshPath(args[0])
	if err != nil || resolved.Device {
		return 0, false
	}
	info, err := os.Stat(resolved.Native)
	if err != nil || !info.IsDir() {
		return 0, false
	}
	return r.changeDirectory("cd", args), true
}

// cdPathTargets is the list `cd` tries for a relative operand, CDPATH first.
//
// POSIX, and the thing that makes `cd src` work from anywhere once CDPATH names the
// parent. An absolute operand, or one beginning with `.` or `..`, skips it entirely --
// those say where they mean, and searching for them would be a way to land somewhere else.
//
// An empty entry, and a CDPATH that is unset, both mean the current directory, so the
// ordinary behaviour survives being at the front or the back of the list.
func (r Runtime) cdPathTargets(target string) []string {
	value := r.vars["CDPATH"]
	if value == "" || isExplicitCDTarget(target) {
		return []string{target}
	}
	candidates := make([]string, 0, 4)
	for _, entry := range filepath.SplitList(value) {
		if entry == "" {
			candidates = append(candidates, target)
			continue
		}
		candidates = append(candidates, trimTrailingSeparator(entry)+"/"+target)
	}
	// The current directory last, so a CDPATH that does not name it still leaves
	// `cd sub` working for a subdirectory that is actually here.
	return append(candidates, target)
}

// isExplicitCDTarget reports whether an operand says where it means on its own.
func isExplicitCDTarget(target string) bool {
	switch {
	case target == "" || target == "-":
		return true
	case target[0] == '/' || target[0] == '\\' || target[0] == '~':
		return true
	case target == "." || target == "..":
		return true
	case len(target) > 1 && target[0] == '.' && (target[1] == '/' || target[1] == '\\'):
		return true
	case len(target) > 2 && target[0] == '.' && target[1] == '.' && (target[2] == '/' || target[2] == '\\'):
		return true
	case len(target) > 1 && target[1] == ':':
		// A drive-qualified path is absolute enough not to be searched for.
		return true
	}
	return false
}

func trimTrailingSeparator(path string) string {
	for len(path) > 1 && (path[len(path)-1] == '/' || path[len(path)-1] == '\\') {
		return path[:len(path)-1]
	}
	return path
}
