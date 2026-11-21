package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

func (r Runtime) pwd() int { return r.printDirectory(r.WorkingDirectory()) }

func (r Runtime) printDirectory(directory string) int {
	_, err := fmt.Fprintln(r.streams.Stdout, directory)
	// A reader that has gone -- `... | head -1` -- ends the output quietly, as every other
	// writer here takes it and busybox does; this said `pwd: pipeline downstream closed`.
	if err != nil && errors.Is(normalizePipelineWriteError(err), errPipelineDownstreamClosed) {
		return 1
	}
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "%spwd: %v\n", r.diagnosticPrefix(), err)
		return 1
	}
	return 0
}

func (r Runtime) export(args []string) int {
	// No operands, or -p, lists what is exported. POSIX specifies the -p form;
	// busybox lists for both, and a shell that prints nothing leaves a user no
	// way to see the environment at all (nemosh issue #10).
	unexport, arrays, args, status, done := r.exportOptions(args)
	if done {
		return status
	}
	if len(args) == 0 && arrays != 0 {
		return r.listExportedArrays(arrays)
	}
	if len(args) == 0 {
		return r.listExported()
	}
	refused := 0
	for _, arg := range args {
		target, value, hasValue := cutAssignment(arg)
		name, appended := splitAssignmentTarget(target)
		if !isValidVariableName(name) && r.BashMode() {
			refused = r.notIdentifier("export", arg)
			continue
		}
		if !isValidVariableName(name) {
			return r.refuseName("export: ", name)
		}
		if !hasValue && unexport {
			r.unexport(name)
			continue
		}
		if !hasValue {
			r.markExported(name)
			continue
		}
		if r.isReadonly(name) {
			return r.refuseReadonly("export: ", name)
		}
		// An array literal makes an exported array, as bash's `declare -ax` would, and `+=(v)`
		// appends to one: `export e=(e x)` held the text (e x), and `x+=(2)` made 1(2).
		if hasValue && r.arrayOperands[arg] {
			if status := r.declareName(context.Background(), declareOptions{export: true}, arg); status != 0 {
				return status
			}
			continue
		}
		// Through assignVar like any assignment -- an attribute applies, `+=`
		// appends -- rather than written into the map, which skipped both.
		if appended {
			value = r.appendedValue(name, value)
		}
		if status := r.assignVar(name, value); status != 0 {
			return status
		}
		// -n assigns and leaves the export as it was, which is busybox's reading.
		if !unexport {
			r.env.Set(name, r.vars[name])
		}
	}
	return refused
}

// unset removes variables, and array elements when the operand carries a subscript.
//
// `unset a[1]` used to return 0 and do nothing at all -- the subscript was never parsed,
// so the whole `a[1]` was looked up as a variable name, found not to exist, and deleting
// a name that is not there succeeds. A script that removed an element and carried on was
// silently wrong, which is the failure mode AGENTS.md singles out: a capability that is
// absent must fail loudly. This one was absent and silent.
//
// `-f` removes functions and `-v` variables, which is also what no option means (POSIX).
// There was no option parsing at all, so `unset -f f` looked for a variable named `-f`, left
// the function defined, and succeeded. A plain `unset f` does not reach a function either,
// which is busybox's reading of POSIX; bash falls back to one.
func (r Runtime) unset(ctx context.Context, args []string) int {
	functions, variables, nameref := false, false, false
	for len(args) > 0 && len(args[0]) > 1 && args[0][0] == '-' {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		for _, letter := range args[0][1:] {
			switch letter {
			case 'f':
				functions = true
			case 'v':
				functions, variables = false, true
			case 'n':
				nameref = true
			default:
				// It ends a script, as busybox's nextopt raises it: unset is a special builtin.
				fmt.Fprintf(r.streams.Stderr, "%sunset: illegal option -%c\n", r.diagnosticPrefix(), letter)
				r.raiseShellError()
				return 2
			}
		}
		args = args[1:]
	}
	// Through a nameref, what it leads to, unless -n asks for the nameref itself.
	if !nameref {
		args = r.unsetTargets(args)
	}
	if functions {
		for _, name := range args {
			if parsed, ok := newFunctionName(name); ok {
				delete(r.functions, parsed)
			}
		}
		return 0
	}
	refused := 0
	for _, name := range args {
		base, subscript, hasSubscript := splitSubscriptedName(name)
		// A name no variable can have is busybox's error, and ends the script as a special
		// builtin's does: `unset a-b` returned 0, having done nothing. In bash mode it is
		// bash's: no error at all with no option, since it may name a function, and with -v
		// not a valid identifier, status 1.
		switch {
		case !isValidVariableName(base) && r.BashMode() && variables:
			refused = r.notIdentifier("unset", name)
			continue
		case !isValidVariableName(base) && r.BashMode():
			continue
		case !isValidVariableName(base):
			return r.refuseName("unset: ", name)
		}
		if r.isReadonly(base) {
			return r.refuseReadonly("unset: ", base)
		}
		if !hasSubscript {
			r.unsetFallingBackToFunction(name, variables)
			continue
		}
		// `a[@]` and `a[*]` remove the whole array, as bash does. Spelling it this way
		// is rare but it is what a script that builds the name does.
		if subscript == "@" || subscript == "*" {
			r.unsetName(base)
			continue
		}
		// An associative name is answered by *key*, so the subscript is a word and not
		// arithmetic: `m[k]` removes the key `k`, where `a[k]` on an indexed array
		// evaluates `k` as a variable and removes that index. Checked before the
		// arithmetic path because `m[0]` is a perfectly good key and would otherwise be
		// read as an index into an array that has none. The key is its subscript expanded as a
		// word, as an assignment's is: `unset 'm[$k]'` and `unset 'm["$k"]'` looked for the
		// key spelled `$k`, quotes and all, and removed nothing.
		if array, ok := r.arrays.associative[base]; ok {
			array.remove(r.resolveKey(ctx, subscript))
			r.markVarMutation(base)
			continue
		}
		// An arithmetic error in it has been said, and ends the script; see resolveSubscript.
		index, err := r.resolveSubscript(ctx, subscript)
		if err != nil {
			return 1
		}
		// A scalar's element 0 is the scalar, as bash reads it, so `unset 'x[0]'` unsets x;
		// any other element of one is bash's error. Both were ignored, status 0.
		if !r.arrays.has(base) {
			if _, set := r.vars[base]; set || r.attributes[base].declared {
				if index != 0 {
					fmt.Fprintf(r.streams.Stderr, "%sunset: %s: not an array variable\n", r.diagnosticPrefix(), base)
					return 1
				}
				r.unsetName(base)
			}
			continue
		}
		// A negative subscript counts back from the last element, so `unset 'a[-1]'` drops
		// it; it removed nothing at all. One that reaches past the start is bash's error.
		index, withinRange := countFromEnd(index, r.arrays.span(base))
		if !withinRange {
			fmt.Fprintf(r.streams.Stderr, "%sunset: [%s]: bad array subscript\n", r.diagnosticPrefix(), subscript)
			return 1
		}
		r.arrays.unsetElement(base, index)
		r.markVarMutation(base)
	}
	return refused
}

// unsetFallingBackToFunction unsets a variable, and in bash mode, given no -v, the function of
// the name where no variable has it, as bash's unset does; busybox's reaches no function.
func (r Runtime) unsetFallingBackToFunction(name string, variables bool) {
	_, set := r.vars[name]
	if r.BashMode() && !variables && !set && !r.isArrayName(name) {
		if parsed, ok := newFunctionName(name); ok {
			delete(r.functions, parsed)
		}
	}
	// A caller's local is taken away rather than unset; see dynamic_unset.go.
	if r.dynamicUnset(name) {
		return
	}
	r.unsetName(name)
}

// splitSubscriptedName separates `a[1]` into its name and its subscript.
//
// The closing bracket must be the last character, so `a[1]x` is a name rather than a
// subscript -- bash refuses that as a bad substitution, and treating it as `a[1]` would
// unset something the writer did not name.
func splitSubscriptedName(text string) (name, subscript string, ok bool) {
	if !strings.HasSuffix(text, "]") {
		return text, "", false
	}
	open := strings.Index(text, "[")
	if open <= 0 {
		return text, "", false
	}
	return text[:open], text[open+1 : len(text)-1], true
}
