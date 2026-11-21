package runtime

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/xiongnemo/nemosh/internal/shellquote"
)

// listExported writes every exported variable as `export NAME='value'`.
//
// Quoted for reuse, which is what POSIX means by writing them "in a form that
// can be reused as input" -- singleQuoteForReuse is the same helper `alias`
// uses, so a value carrying a quote comes back correct from either.
//
// Sorted by name, so two runs of the same script produce the same output and a
// difference between them means something.
func (r Runtime) listExported() int {
	entries := r.env.Environ()
	sort.Strings(entries)
	for _, entry := range entries {
		name, value, found := strings.Cut(entry, "=")
		if !found || name == "" {
			continue
		}
		fmt.Fprintf(r.streams.Stdout, "export %s=%s\n", name, shellquote.Ash(value))
	}
	for _, name := range r.pendingExports() {
		fmt.Fprintf(r.streams.Stdout, "export %s\n", name)
	}
	return 0
}

// listExportedArrays is bash's `export -a` or `export -A` with no names: each exported array of
// the kind, in declare's form. An array never reaches the environment, in bash or here.
func (r Runtime) listExportedArrays(kind rune) int {
	var names []string
	if kind == 'A' {
		names = slices.Sorted(maps.Keys(r.arrays.associative))
	} else {
		names = r.arrays.indexedNames()
		slices.Sort(names)
	}
	for _, name := range names {
		if !r.isExported(name) {
			continue
		}
		if text, ok := r.declarationText(name); ok {
			fmt.Fprintln(r.streams.Stdout, text)
		}
	}
	return 0
}
