package runtime

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// `declare` and `typeset` -- one name for two spellings, as in bash and ksh.
//
// It was not a builtin at all, so `declare -A m` was a command lookup that failed
// with `declare: not found`, and with it went associative arrays: there is no other
// way to say a name is one.
//
// What is accepted is what this shell can honour. The options it cannot are refused
// by name rather than ignored, because an ignored `-i` leaves a variable that is not
// an integer and a script that believes it is.

// declareBuiltin is `declare [-aArxp] [name[=value] ...]`.
func (r Runtime) declareBuiltin(ctx context.Context, args []string) int {
	options, names, err := parseDeclareOptions(args)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "declare: %v\n", err)
		return 2
	}
	if options.print || len(names) == 0 {
		r.printDeclarations(names)
		return 0
	}
	for _, name := range names {
		if status := r.declareName(ctx, options, name); status != 0 {
			return status
		}
	}
	return 0
}

type declareOptions struct {
	associative bool
	indexed     bool
	readonly    bool
	export      bool
	print       bool
}

func parseDeclareOptions(args []string) (declareOptions, []string, error) {
	var options declareOptions
	index := 0
	for ; index < len(args); index++ {
		argument := args[index]
		if argument == "--" {
			index++
			break
		}
		if len(argument) < 2 || argument[0] != '-' {
			break
		}
		for _, letter := range argument[1:] {
			switch letter {
			case 'A':
				options.associative = true
			case 'a':
				options.indexed = true
			case 'r':
				options.readonly = true
			case 'x':
				options.export = true
			case 'p':
				options.print = true
			case 'g':
				// Every declaration here is global, because `local` is what makes a
				// name local and this shell has it separately. So -g asks for what
				// already happens.
			default:
				return options, nil, fmt.Errorf(
					"-%c: not an option this build has; it takes -A -a -r -x -p -g", letter)
			}
		}
	}
	if options.associative && options.indexed {
		return options, nil, fmt.Errorf("-A and -a cannot both be given: a name is one kind or the other")
	}
	return options, args[index:], nil
}

// declareName applies one `name` or `name=value`.
func (r Runtime) declareName(ctx context.Context, options declareOptions, argument string) int {
	name, value, assigned := strings.Cut(argument, "=")
	if reference, ok := parseArrayReference(name); ok {
		// `declare m[k]=v` is not something to encourage, but it is what an
		// element assignment looks like and refusing it here would be arbitrary.
		if !assigned {
			return 0
		}
		return r.assignElementByKind(ctx, reference, value)
	}
	if !isValidVariableName(name) {
		fmt.Fprintf(r.streams.Stderr, "declare: %s: not a valid name\n", name)
		return 1
	}
	switch {
	case options.associative:
		r.arrays.declareAssociative(name)
	case options.indexed:
		if _, exists := r.arrays.get(name); !exists {
			r.arrays.set(name, nil)
		}
	}
	if assigned {
		// `declare -a x=(one two)`. The lexer keeps the parenthesised list in one
		// word -- the `(` follows `x=`, which is the test it applies -- so it
		// arrives here whole and has to be split into elements. Without this it
		// became the single string `(one two)`.
		if inner, ok := parenthesisedList(value); ok {
			r.arrays.set(name, r.expandArrayElements(ctx, inner, 0))
			r.syncArrayScalar(name)
		} else if status := r.assignVar(name, value); status != 0 {
			return status
		}
	}
	if options.export {
		r.env.Set(name, r.vars[name])
	}
	if options.readonly {
		// The same set `readonly` writes to, so a name made read-only either way is
		// refused by the one check in assignVar.
		r.readonly[name] = struct{}{}
	}
	return 0
}

// assignElementByKind writes `m[k]=v`, choosing between a key and an index by what
// the name was declared as. The distinction is the whole point of `declare -A`:
// without it `m[k]` is an arithmetic subscript and `k` is a variable holding a
// number.
func (r Runtime) assignElementByKind(ctx context.Context, reference arrayReference, value string) int {
	if r.arrays.isAssociative(reference.name) {
		r.arrays.setKey(reference.name, r.resolveKey(ctx, reference.subscript), value)
		return 0
	}
	return r.assignArrayElementText(ctx, reference, value)
}

// printDeclarations is `declare -p`, and `declare` with no operands.
//
// The form is bash's, because it is meant to be read back by the shell: a `declare -p`
// whose output cannot be pasted into a script is a listing, not a declaration.
func (r Runtime) printDeclarations(names []string) {
	if len(names) > 0 {
		for _, name := range names {
			r.printOneDeclaration(strings.SplitN(name, "=", 2)[0])
		}
		return
	}
	for _, name := range r.arrays.associativeNames() {
		r.printOneDeclaration(name)
	}
	scalars := make([]string, 0, len(r.vars))
	for name := range r.vars {
		scalars = append(scalars, name)
	}
	sort.Strings(scalars)
	for _, name := range scalars {
		if r.arrays.isAssociative(name) {
			continue
		}
		r.printOneDeclaration(name)
	}
}

func (r Runtime) printOneDeclaration(name string) {
	if r.arrays.isAssociative(name) {
		var out strings.Builder
		fmt.Fprintf(&out, "declare -A %s=(", name)
		for _, key := range r.arrays.keysOf(name) {
			value, _ := r.arrays.lookupKey(name, key)
			fmt.Fprintf(&out, "[%s]=%q ", key, value)
		}
		fmt.Fprintln(r.streams.Stdout, strings.TrimSuffix(out.String(), " ")+")")
		return
	}
	if elements, ok := r.arrays.get(name); ok {
		var out strings.Builder
		fmt.Fprintf(&out, "declare -a %s=(", name)
		for index, element := range elements {
			fmt.Fprintf(&out, "[%d]=%q ", index, element)
		}
		fmt.Fprintln(r.streams.Stdout, strings.TrimSuffix(out.String(), " ")+")")
		return
	}
	value, set := r.vars[name]
	if !set {
		fmt.Fprintf(r.streams.Stderr, "declare: %s: not found\n", name)
		return
	}
	fmt.Fprintf(r.streams.Stdout, "declare -- %s=%q\n", name, value)
}

// parenthesisedList reports the inside of a `(one two)` array literal.
func parenthesisedList(value string) (string, bool) {
	if !strings.HasPrefix(value, "(") || !strings.HasSuffix(value, ")") {
		return "", false
	}
	return value[1 : len(value)-1], true
}
