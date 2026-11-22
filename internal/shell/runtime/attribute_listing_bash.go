package runtime

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/xiongnemo/nemosh/internal/shellquote"
)

// bashAttributeListing is readonly's or export's list in bash mode, builtin naming which, as
// bash writes it: declare's line for each name outside its POSIX mode, `declare -r q="it's"`,
// `declare -x v="333"`, `declare -ar arr=(...)`; under it the builtin's own name and declare's
// quoting, `readonly q="it's"`, `readonly -a arr=(...)`, `export v="333"`, `export w`. Measured
// against Git for Windows' bash 5.3. busybox's, which nemosh and sh keep, single-quote the value
// its own way, `readonly q='it'"'"'s'`.
func (r Runtime) bashAttributeListing(builtin string, names []string) int {
	for _, name := range names {
		text, ok := r.declarationText(name)
		if value, dynamic := r.dynamicParameter(name); dynamic && !strings.Contains(text, "=") {
			// SHELLOPTS and BASHOPTS are computed; see shellopts.go.
			text, ok = fmt.Sprintf("declare -%s %s=%s", r.declareFlags(name), name, shellquote.Double(value)), true
		}
		if !ok {
			continue
		}
		if r.options.posix {
			flags, rest, _ := strings.Cut(strings.TrimPrefix(text, "declare -"), " ")
			prefix := builtin + " "
			switch {
			case strings.Contains(flags, "A"):
				prefix += "-A "
			case strings.Contains(flags, "a"):
				prefix += "-a "
			}
			text = prefix + rest
		}
		fmt.Fprintln(r.streams.Stdout, text)
	}
	return 0
}

// bashExportedNames is every exported variable a script can name, in order, for export's list in
// bash mode: those with a value in the environment and those exported with none yet.
func (r Runtime) bashExportedNames() []string {
	seen := map[string]bool{}
	for _, entry := range r.env.Environ() {
		if name, _, found := strings.Cut(entry, "="); found && isValidVariableName(name) {
			seen[name] = true
		}
	}
	for _, name := range r.pendingExports() {
		seen[name] = true
	}
	return slices.Sorted(maps.Keys(seen))
}
