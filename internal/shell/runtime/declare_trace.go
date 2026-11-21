package runtime

// `declare -t` is bash's trace attribute. A function that has it inherits the DEBUG and RETURN
// traps, as every function does under `set -T` -- bash-preexec gives its hooks the attribute so
// the DEBUG trap they set is the shell's -- and keeps it when it is defined again, the attribute
// being the name's. On a variable it means nothing, and `declare -p` shows it. `declare -ft
// NAME` sets it on a function and `declare -f +t NAME` takes it away; a name that is no function
// is status 1, and said nothing of, as bash has it. busybox has no declare.

// traceFunctions sets or takes away the trace attribute of each function named.
func (r Runtime) traceFunctions(names []string, traced bool) int {
	status := 0
	for _, name := range names {
		parsed, ok := newFunctionName(name)
		definition, found := r.functions[parsed]
		if !ok || !found {
			status = 1
			continue
		}
		definition.traced = traced
		r.functions[parsed] = definition
	}
	return status
}

// keepTrace is a definition replacing another of the same name, which keeps its trace
// attribute.
func (r Runtime) keepTrace(definition functionDefinition) functionDefinition {
	if old, found := r.functions[definition.name]; found && old.traced {
		definition.traced = true
	}
	return definition
}

// tracedFunctionNames are the functions that have the attribute, which a job process is given.
func (r Runtime) tracedFunctionNames() []string {
	var names []string
	for _, name := range r.sortedFunctionNames() {
		if r.functions[functionName{value: name}].traced {
			names = append(names, name)
		}
	}
	return names
}
