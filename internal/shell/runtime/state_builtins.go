package runtime

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/xiongnemo/nemosh/internal/pathmodel"
)

func (r Runtime) pwd() int {
	_, err := fmt.Fprintln(r.streams.Stdout, r.WorkingDirectory())
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "pwd: %v\n", err)
		return 1
	}
	return 0
}

func (r Runtime) export(args []string) int {
	for _, arg := range args {
		name, value, hasValue := strings.Cut(arg, "=")
		if name == "" {
			return 2
		}
		if !hasValue {
			value = r.vars[name]
		} else if r.isReadonly(name) {
			fmt.Fprintf(r.streams.Stderr, "export: %s: readonly variable\n", name)
			return 1
		}
		r.vars[name] = value
		r.env.Set(name, value)
		r.markVarMutation(name)
	}
	return 0
}

func (r Runtime) unset(args []string) int {
	for _, name := range args {
		if r.isReadonly(name) {
			fmt.Fprintf(r.streams.Stderr, "unset: %s: readonly variable\n", name)
			return 1
		}
		delete(r.vars, name)
		r.env.Unset(name)
		r.markVarMutation(name)
	}
	return 0
}

// cd follows busybox ash's cdcmd (shell/ash.c:11808): no operand goes to HOME,
// a lone `-` goes to OLDPWD and prints where it landed, and both PWD and OLDPWD
// are updated and exported afterwards (setpwd, shell/ash.c:3571,3591). Nemosh
// kept none of that: bare `cd` stayed put, `-` was looked up as a directory
// name, and $PWD held whatever the process started with -- a stale value that
// was then handed to every child.
func (r Runtime) cd(args []string) int {
	target, printResult, ok := r.cdTarget(args)
	if !ok {
		return 1
	}
	resolved, err := r.ResolveNemoshPath(target)
	if err != nil {
		// A host-only UNC path is not a missing directory, it is not a
		// directory at all, so the failure reads as one line of the ordinary
		// shape plus a line naming what to type instead. The hint comes from
		// the path model rather than from the operand, so `//server/` advises
		// `//server/share` and not `//server//share`, and a hostless `//` --
		// which the model calls malformed, not host-only -- gets no share
		// suggestion it cannot complete.
		var hostOnly pathmodel.HostOnlyUNCError
		if errors.As(err, &hostOnly) {
			fmt.Fprintf(r.streams.Stderr, "cd: %s: No such file or directory\n", target)
			fmt.Fprintf(r.streams.Stderr, "hint: %v\n", hostOnly)
			return 1
		}
		fmt.Fprintf(r.streams.Stderr, "cd: %s: %v\n", target, err)
		return 1
	}
	if resolved.Device {
		fmt.Fprintf(r.streams.Stderr, "cd: %s: not a directory\n", target)
		return 1
	}
	info, err := os.Stat(resolved.Native)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "cd: %s: %v\n", target, err)
		return 1
	}
	// Not folded into the branch above: with a nil error there was nothing to
	// format, so a `cd` onto a regular file reported the literal text <nil> as
	// its reason.
	if !info.IsDir() {
		fmt.Fprintf(r.streams.Stderr, "cd: %s: Not a directory\n", target)
		return 1
	}
	previous := r.WorkingDirectory()
	r.paths.setWorkingDirectory(resolved)
	r.setDirectoryVariable("OLDPWD", previous)
	r.setDirectoryVariable("PWD", r.WorkingDirectory())
	if printResult {
		fmt.Fprintln(r.streams.Stdout, r.WorkingDirectory())
	}
	return 0
}

// cdTarget answers where to go, whether to print it on arrival, and whether the
// operands made sense at all.
func (r Runtime) cdTarget(args []string) (string, bool, bool) {
	if len(args) == 0 {
		home := r.vars["HOME"]
		if home == "" {
			fmt.Fprintln(r.streams.Stderr, "cd: HOME not set")
			return "", false, false
		}
		return home, false, true
	}
	if args[0] != "-" {
		return args[0], false, true
	}
	previous := r.vars["OLDPWD"]
	if previous == "" {
		fmt.Fprintln(r.streams.Stderr, "cd: OLDPWD not set")
		return "", false, false
	}
	return previous, true, true
}

// PWD and OLDPWD are exported the way busybox sets them, because their whole
// use is to be read by something else -- a prompt, a subshell, a child process.
func (r Runtime) setDirectoryVariable(name, value string) {
	r.vars[name] = value
	r.env.Set(name, value)
	r.markVarMutation(name)
}
