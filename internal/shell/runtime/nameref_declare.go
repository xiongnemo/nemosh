package runtime

import "fmt"

// declareNameref is `declare -n name[=target]`, and `local -n`. The value is stored as it is
// written -- it is the name the reference leads to, not a value to write through it -- and a
// value that is not a name is refused, leaving name as it was, as bash refuses it: `ref=1;
// typeset -n ref` is an error and ref stays the string 1. A read-only nameref keeps its
// target; what it leads to can still be written through it, in bash too.
func (r Runtime) declareNameref(name, value string, assigned bool) int {
	current, set := value, assigned
	if !assigned {
		current, set = r.vars[name]
	}
	if set && !isNamerefValue(current) {
		fmt.Fprintf(r.streams.Stderr, "%sdeclare: `%s': invalid variable name for name reference\n", r.diagnosticPrefix(), current)
		return 1
	}
	if assigned && r.isReadonly(name) {
		return r.refuseReadonly("declare: ", name)
	}
	attributes := r.attributes[name]
	attributes.nameref = true
	r.attributes[name] = attributes
	if assigned {
		r.vars[name] = value
		if r.isExported(name) {
			r.env.Set(name, value)
		}
	}
	r.markVarMutation(name)
	return 0
}
