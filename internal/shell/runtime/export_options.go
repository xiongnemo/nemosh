package runtime

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/xiongnemo/nemosh/internal/shellquote"
)

// export's and readonly's options.
//
// `export -n name` takes name out of the environment and keeps its value, as busybox's does.
// It was "export: -n: bad variable name", a shell error, so a script that meant to stop
// passing something on ended instead. `-f`, bash's exported function, is busybox's "illegal
// option", which ends a script there too. `readonly` with no names, or -p, lists the read-only
// names in busybox's form, `readonly NAME='value'`, and printed nothing; `readonly -a` and
// `-A` declare a read-only array, as bash's do.

// exportOptions reads export's leading options: -n to take names out of the environment, -p
// to list. done reports that export has answered, with status.
func (r Runtime) exportOptions(args []string) (unexport bool, operands []string, status int, done bool) {
	for len(args) > 0 && len(args[0]) > 1 && args[0][0] == '-' {
		if args[0] == "--" {
			return unexport, args[1:], 0, false
		}
		for _, letter := range args[0][1:] {
			switch letter {
			case 'n':
				unexport = true
			case 'p':
			default:
				fmt.Fprintf(r.streams.Stderr, "%sexport: illegal option -%c\n", r.diagnosticPrefix(), letter)
				r.raiseShellError()
				return false, nil, 2, true
			}
		}
		args = args[1:]
	}
	return unexport, args, 0, false
}

// readonlyOptions reads readonly's leading options: -p to list, -a and -A for an array, as
// `declare -ar` and `declare -Ar` have it.
func (r Runtime) readonlyOptions(args []string) (declareOptions, []string, bool) {
	options := declareOptions{readonly: true}
	for len(args) > 0 && len(args[0]) > 1 && args[0][0] == '-' {
		if args[0] == "--" {
			return options, args[1:], true
		}
		for _, letter := range args[0][1:] {
			switch letter {
			case 'a':
				options.indexed = true
			case 'A':
				options.associative = true
			case 'p':
			default:
				fmt.Fprintf(r.streams.Stderr, "%sreadonly: illegal option -%c\n", r.diagnosticPrefix(), letter)
				r.raiseShellError()
				return options, nil, false
			}
		}
		args = args[1:]
	}
	return options, args, true
}

// listReadonly writes every read-only name as busybox does: `readonly NAME='value'`, or
// `readonly NAME` for one with no value; an array in declare's form.
func (r Runtime) listReadonly() int {
	for _, name := range slices.Sorted(maps.Keys(r.readonly)) {
		if r.arrays.isAssociative(name) || r.hasIndexedArray(name) {
			if text, ok := r.declarationText(name); ok {
				fmt.Fprintln(r.streams.Stdout, text)
			}
			continue
		}
		value, set := r.vars[name]
		if !set {
			// SHELLOPTS and BASHOPTS are computed; see shellopts.go.
			value, set = r.dynamicParameter(name)
		}
		if set {
			fmt.Fprintf(r.streams.Stdout, "readonly %s=%s\n", name, shellquote.Ash(value))
			continue
		}
		fmt.Fprintf(r.streams.Stdout, "readonly %s\n", name)
	}
	return 0
}

// readonlyArrays is `readonly -a name=(...)` and `-A`: each operand declared as declare -ar
// or -Ar declares it.
func (r Runtime) readonlyArrays(ctx context.Context, options declareOptions, operands []string) int {
	for _, operand := range operands {
		target, _, _ := cutAssignment(operand)
		if name, _ := splitAssignmentTarget(target); !isValidVariableName(name) {
			return r.refuseName("readonly: ", name)
		}
		if status := r.declareName(ctx, options, operand); status != 0 {
			return status
		}
	}
	return 0
}
