package runtime

import (
	"io"
	"sort"
)

// sortedFunctionNames is every function, in the order `declare -f` lists them.
func (r Runtime) sortedFunctionNames() []string {
	names := make([]string, 0, len(r.functions))
	for name := range r.functions {
		names = append(names, name.value)
	}
	sort.Strings(names)
	return names
}

// declareFunctionBodies is `declare -f [name...]`: each definition, printed back from what
// was parsed (scriptPrinter), all of them in name order when none is named. A name that is
// not a function is status 1 and nothing printed, as in bash. It was not an option.
func (r Runtime) declareFunctionBodies(names []string) int {
	if len(names) == 0 {
		names = r.sortedFunctionNames()
	}
	status := 0
	for _, name := range names {
		parsed, ok := newFunctionName(name)
		definition, found := r.functions[parsed]
		if !ok || !found {
			status = 1
			continue
		}
		io.WriteString(r.streams.Stdout, printFunction(definition))
	}
	return status
}
