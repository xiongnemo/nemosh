package runtime

import (
	"fmt"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/pathmodel"
)

// cd and pwd take -L and -P, as busybox's do. -L, the default, keeps a path as it was
// reached; -P follows every link in it, junctions included, so `cd -P link` lands in the
// directory the link leads to and `pwd -P` names it. The last one given wins, `--` ends them,
// and `set -P` makes -P the default, as in bash. They were taken for directory names:
// `cd -P dir` was "cd: -P: No such file or directory", and `pwd -P` printed the logical path.

// directoryOptions reads the leading -L, -P and `--`, and answers whether the path is to be
// physical and what is left.
func (r Runtime) directoryOptions(as string, args []string) (bool, []string, bool) {
	physical := r.options.physical
	for len(args) > 0 {
		arg := args[0]
		if arg == "--" {
			return physical, args[1:], true
		}
		if len(arg) < 2 || arg[0] != '-' {
			break
		}
		for _, letter := range arg[1:] {
			switch letter {
			case 'L':
				physical = false
			case 'P':
				physical = true
			default:
				fmt.Fprintf(r.streams.Stderr, "%s: illegal option -%c\n", as, letter)
				return false, nil, false
			}
		}
		args = args[1:]
	}
	return physical, args, true
}

// physicalPath is a directory with every link in it followed, spelled as the shell spells a
// path. One it cannot follow is left as it was.
func (r Runtime) physicalPath(resolved pathmodel.ResolvedPath) pathmodel.ResolvedPath {
	followed, err := applets.FollowLinks(resolved.Native)
	if err != nil {
		return resolved
	}
	physical, err := r.ResolveNemoshPath(followed)
	if err != nil || physical.Device {
		return resolved
	}
	return r.withRealCase(physical)
}

// pwdBuiltin is `pwd [-L|-P]`.
func (r Runtime) pwdBuiltin(args []string) int {
	// An operand is ignored, as both references ignore it.
	physical, _, ok := r.directoryOptions("pwd", args)
	if !ok {
		return 2
	}
	if !physical {
		return r.pwd()
	}
	resolved, err := r.ResolveNemoshPath(r.WorkingDirectory())
	if err != nil {
		return r.pwd()
	}
	return r.printDirectory(string(r.physicalPath(resolved).Canonical))
}

// cdTarget answers where to go, whether to print it on arrival, and whether the
// operands made sense at all.
//
// An empty directory is the current one, as busybox's cdcmd has it (`if (!*dest) dest = "."`):
// `cd ""`, and an empty HOME or OLDPWD, which bash takes the same way. pushd is bash's own, and
// refuses an empty operand as a null directory, as bash does.
func (r Runtime) cdTarget(as string, args []string) (string, bool, bool) {
	if len(args) == 0 {
		home, set := r.vars["HOME"]
		if !set {
			fmt.Fprintln(r.streams.Stderr, fmt.Sprintf("%s: HOME not set", as))
			return "", false, false
		}
		return currentIfEmpty(home), false, true
	}
	if args[0] == "" && as != "cd" {
		fmt.Fprintln(r.streams.Stderr, fmt.Sprintf("%s: null directory", as))
		return "", false, false
	}
	if args[0] != "-" {
		return currentIfEmpty(args[0]), false, true
	}
	previous, set := r.vars["OLDPWD"]
	if !set {
		fmt.Fprintln(r.streams.Stderr, fmt.Sprintf("%s: OLDPWD not set", as))
		return "", false, false
	}
	return currentIfEmpty(previous), true, true
}

// currentIfEmpty is cd's reading of an empty directory as the current one.
func currentIfEmpty(directory string) string {
	if directory == "" {
		return "."
	}
	return directory
}
