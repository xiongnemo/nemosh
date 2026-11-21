package runtime

import (
	"fmt"
	"strconv"
	"strings"
)

// Definitions are what a startup file can define, each by the name a person gives it -- PS1,
// alias ll, function f, set -o vi, shopt extglob, complete git -- with the text that defines
// it. A session in bash mode reads bash's startup files and then nemosh's, and says which of
// the names the first set the second changed; cmd/nemosh compares three of these to know.
type Definitions map[string]string

// Definitions is the shell's definitions as they stand. What the shell keeps for itself as it
// runs, $_ and the statuses and matches of the last command, is left out.
func (r Runtime) Definitions() Definitions {
	defined := Definitions{}
	for name, value := range r.vars {
		if !shellKept(name) {
			defined[name] = value
		}
	}
	if r.arrays != nil {
		for _, name := range r.arrays.indexedNames() {
			if !shellKept(name) {
				defined[name] = strings.Join(r.arrays.liveValues(name), "\x00")
			}
		}
		for name, array := range r.arrays.associative {
			var text strings.Builder
			for _, key := range array.keys() {
				text.WriteString(key + "\x00" + array.entries[key] + "\x00")
			}
			defined[name] = text.String()
		}
	}
	for name, value := range r.aliases {
		defined["alias "+name] = value
	}
	for name, definition := range r.functions {
		defined["function "+name.value] = printFunction(definition)
	}
	for _, spec := range shellOptionSpecs {
		if spec.name != "" && spec.field != nil {
			defined["set -o "+spec.name] = strconv.FormatBool(*spec.field(r.options))
		}
	}
	for _, option := range shoptOptions {
		if option.field != nil {
			defined["shopt "+option.name] = strconv.FormatBool(*option.field(r.options))
		}
	}
	if r.completions != nil {
		for name, spec := range r.completions.specs {
			defined["complete "+name] = fmt.Sprint(spec)
		}
	}
	return defined
}

// shellKept is a variable the shell sets for itself as it runs rather than one a file defines.
func shellKept(name string) bool {
	return name == "_" || name == "PIPESTATUS" || name == "BASH_REMATCH"
}
