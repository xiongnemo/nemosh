package runtime

import (
	"fmt"
	"path/filepath"
)

// typeBuiltin implements POSIX `type`: for each name, say how the shell would
// interpret it. It answers from the same lookup dispatch uses, so it cannot
// disagree with what actually runs, and it reports every name given rather than
// stopping at the first failure.
func (r Runtime) typeBuiltin(args []string) int {
	if len(args) == 0 {
		return 0
	}
	status := 0
	for _, name := range args {
		description, ok := r.describeCommand(name)
		if !ok {
			fmt.Fprintf(r.streams.Stderr, "type: %s: not found\n", name)
			status = 1
			continue
		}
		fmt.Fprintln(r.streams.Stdout, description)
	}
	return status
}

// The order matches lookup: an alias first, then a shell builtin, then a
// function, then an applet, then PATH.
func (r Runtime) describeCommand(name string) (string, bool) {
	if value, ok := r.aliases[name]; ok {
		return fmt.Sprintf("%s is an alias for %s", name, value), true
	}
	if isRuntimeBuiltin(name) {
		return name + " is a shell builtin", true
	}
	if parsed, ok := newFunctionName(name); ok {
		if _, found := r.functions[parsed]; found {
			return name + " is a function", true
		}
	}
	if _, ok := r.lookupApplet(name); ok {
		return name + " is a bundled applet", true
	}
	resolved, err := r.externalCommandPath(name)
	if err != nil {
		return "", false
	}
	return fmt.Sprintf("%s is %s", name, filepath.ToSlash(resolved)), true
}
