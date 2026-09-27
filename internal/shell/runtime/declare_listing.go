package runtime

import (
	"fmt"
	"sort"
	"strings"

	"github.com/xiongnemo/nemosh/internal/shellquote"
)

// listDeclarations is `declare` and `typeset` with no names, bash's three listings:
//
//   - with no options, every variable bare, `x='a b'` as bash's set writes it, and the
//     functions after them;
//   - with -p, every variable as the declaration that recreates it;
//   - with attributes, -x or -ir, the variables that have any of them, the same way.
//
// Sorted by name, with the arrays among the rest. Each of the three printed every variable
// as declare -p writes it, whatever was asked for, and the associative arrays first.
func (r Runtime) listDeclarations(options declareOptions) int {
	wanted := options.attributeLetters()
	bare := !options.print && wanted == ""
	for _, name := range r.declaredNames() {
		if bare {
			r.printBareDeclaration(name)
			continue
		}
		if wanted != "" && !strings.ContainsAny(r.declareFlags(name), wanted) {
			continue
		}
		if text, found := r.declarationText(name); found {
			fmt.Fprintln(r.streams.Stdout, text)
		}
	}
	if bare {
		return r.declareFunctionBodies(nil)
	}
	return 0
}

// attributeLetters are the attributes asked for, as declareFlags writes them.
func (o declareOptions) attributeLetters() string {
	var letters strings.Builder
	for _, attribute := range []struct {
		on     bool
		letter byte
	}{
		{o.associative, 'A'}, {o.indexed, 'a'}, {o.integer, 'i'}, {o.nameref, 'n'},
		{o.readonly, 'r'}, {o.export, 'x'}, {o.lower, 'l'}, {o.upper, 'u'},
	} {
		if attribute.on {
			letters.WriteByte(attribute.letter)
		}
	}
	return letters.String()
}

// declaredNames is every variable the shell has, sorted: those with values, arrays of both
// kinds, and names declared with attributes and no value yet.
func (r Runtime) declaredNames() []string {
	seen := map[string]bool{}
	for name := range r.vars {
		seen[name] = true
	}
	for _, name := range r.arrays.associativeNames() {
		seen[name] = true
	}
	for name := range r.attributes {
		seen[name] = true
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// printBareDeclaration is one variable of the bare listing, as bash writes it: `name=value`,
// quoted only as much as it needs, and an array as the list declare -p gives it. A name
// with no value, and an array with no element, are not listed.
func (r Runtime) printBareDeclaration(name string) {
	switch {
	case r.arrays.isAssociative(name):
		if len(r.arrays.keysOf(name)) == 0 {
			return
		}
	case r.hasIndexedArray(name):
		if len(r.arrays.liveIndices(name)) == 0 {
			return
		}
	default:
		if value, set := r.vars[name]; set {
			fmt.Fprintf(r.streams.Stdout, "%s=%s\n", name, shellquote.Reusable(value))
		}
		return
	}
	// `declare -a arr=(...)` without its `declare -a `.
	text, _ := r.declarationText(name)
	if parts := strings.SplitN(text, " ", 3); len(parts) == 3 {
		fmt.Fprintln(r.streams.Stdout, parts[2])
	}
}
