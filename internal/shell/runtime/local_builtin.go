package runtime

import (
	"fmt"
	"strings"
)

// local declares variables that belong to the function call that runs it, so
// the value they had outside comes back when the call returns.
//
// It is not in POSIX -- 2.9.5 makes every variable a function touches global --
// but ash, dash and bash all have it and nearly every real script leans on it,
// so a shell without it silently leaks working variables into its caller.
// busybox carries it as BUILTIN_SPEC_REG_ASSG (shell/ash.c:12101).
//
// Outside a function there is nothing to restore to, and the shells report that
// rather than quietly behaving like an assignment.
func (r Runtime) local(args []string) int {
	if r.functionDepth == 0 || r.locals == nil {
		fmt.Fprintln(r.streams.Stderr, "local: not in a function")
		return 1
	}
	status := 0
	for _, arg := range args {
		name, value, assigns := strings.Cut(arg, "=")
		if !isVariableName(name) {
			fmt.Fprintf(r.streams.Stderr, "local: %s: bad variable name\n", name)
			status = 2
			continue
		}
		if r.isReadonly(name) {
			fmt.Fprintf(r.streams.Stderr, "local: %s: readonly variable\n", name)
			status = 1
			continue
		}
		r.locals.save(name, r.vars)
		if !assigns {
			// `local x` with no value leaves x unset for the call, which is
			// what makes it a declaration rather than an assignment.
			delete(r.vars, name)
			continue
		}
		r.vars[name] = value
		r.markVarMutation(name)
	}
	return status
}

// localScope remembers what each name held before a function call shadowed it.
// One per call; a nested call gets its own, so restoring unwinds in the order
// the calls did.
type localScope struct {
	saved map[string]savedVariable
}

type savedVariable struct {
	value   string
	present bool
}

func newLocalScope() *localScope {
	return &localScope{saved: map[string]savedVariable{}}
}

// save records a name's outer value once. Twice would overwrite the outer value
// with the local one, so `local x=1; local x=2` would restore 1 rather than
// whatever the caller had.
func (s *localScope) save(name string, vars map[string]string) {
	if _, already := s.saved[name]; already {
		return
	}
	value, present := vars[name]
	s.saved[name] = savedVariable{value: value, present: present}
}

func (s *localScope) restore(vars map[string]string) {
	for name, previous := range s.saved {
		if previous.present {
			vars[name] = previous.value
			continue
		}
		delete(vars, name)
	}
}
