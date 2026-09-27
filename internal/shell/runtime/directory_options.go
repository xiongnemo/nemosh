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
