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

func (r Runtime) cd(args []string) int {
	target := "."
	if len(args) > 0 {
		target = args[0]
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
	if err != nil || !info.IsDir() {
		fmt.Fprintf(r.streams.Stderr, "cd: %s: %v\n", target, err)
		return 1
	}
	r.paths.setWorkingDirectory(resolved)
	return 0
}
