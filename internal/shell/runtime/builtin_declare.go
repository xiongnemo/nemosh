package runtime

import (
	"context"
	"fmt"
	"strings"

	"github.com/xiongnemo/nemosh/internal/shellquote"
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
	if options.functionNames {
		return r.declareFunctionNames(names)
	}
	if options.functionBodies {
		return r.declareFunctionBodies(names)
	}
	if len(names) == 0 {
		return r.listDeclarations(options)
	}
	if options.print {
		return r.printDeclarations(names)
	}
	for _, name := range names {
		if status := r.declareInFunction(ctx, options, name); status != 0 {
			return status
		}
	}
	return 0
}

// declareInFunction is declare's scope rule, bash's: inside a function a name it declares
// is the call's own, as `local` makes it, unless -g says otherwise. Every declaration here
// was global, so `f() { declare x=1; }` left x set after f returned, and `typeset` -- the
// ksh spelling a script uses for a local -- did the same.
func (r Runtime) declareInFunction(ctx context.Context, options declareOptions, argument string) int {
	target, _, _ := cutAssignment(argument)
	if name, _ := splitAssignmentTarget(target); options.global || r.functionDepth == 0 || r.locals == nil || !isVariableName(assignedName(name)) {
		return r.declareName(ctx, options, argument)
	}
	return r.declareLocal(ctx, "declare", options, argument)
}

type declareOptions struct {
	associative bool
	indexed     bool
	readonly    bool
	export      bool
	print       bool
	// integer, lower and upper are -i -l -u; the removed set is the `+i +l +u +x`
	// that takes an attribute away again.
	integer, lower, upper bool
	removed               string
	// functionNames is -F; see declareFunctionNames. functionBodies is -f, the
	// definitions themselves; see declareFunctionBodies.
	functionNames  bool
	functionBodies bool
	// global is -g: inside a function the names are the caller's rather than the call's.
	global bool
	// nameref is -n; see nameref.go.
	nameref bool
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
		if len(argument) >= 2 && argument[0] == '+' {
			for _, letter := range argument[1:] {
				if !strings.ContainsRune("ilunx", letter) {
					return options, nil, fmt.Errorf("+%c: not an attribute this build can take away; it takes +i +l +u +n +x", letter)
				}
			}
			options.removed += argument[1:]
			continue
		}
		if len(argument) < 2 || argument[0] != '-' {
			break
		}
		for _, letter := range argument[1:] {
			switch letter {
			case 'F':
				options.functionNames = true
			case 'f':
				options.functionBodies = true
			case 'i':
				options.integer = true
			case 'l':
				options.lower, options.upper = true, false
			case 'u':
				options.upper, options.lower = true, false
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
				options.global = true
			case 'n':
				options.nameref = true
			default:
				return options, nil, fmt.Errorf(
					"-%c: not an option this build has; it takes -A -a -i -l -u -n -r -x -p -g -f -F", letter)
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
	target, value, assigned := cutAssignment(argument)
	name, appended := splitAssignmentTarget(target)
	if reference, ok := parseArrayReference(name); ok {
		// `declare m[k]=v` is not something to encourage, but it is what an
		// element assignment looks like and refusing it here would be arbitrary.
		if !assigned {
			return 0
		}
		if appended {
			value = r.appendedValue(name, value)
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
		if !r.arrays.has(name) {
			r.arrays.set(name, nil)
		}
	}
	// Before the value, so `declare -i n=2+3` stores 5.
	r.applyDeclaredAttributes(name, options)
	if options.nameref {
		if status := r.declareNameref(name, value, assigned); status != 0 {
			return status
		}
	} else if assigned {
		// `declare -a x=(one two)`. The lexer keeps the parenthesised list in one
		// word -- the `(` follows `x=`, which is the test it applies -- so it
		// arrives here whole and has to be split into elements. Without this it
		// became the single string `(one two)`. Only a literal written so is a list,
		// or any text like one under -a or -A: `declare x='(a b)'` is a string in
		// bash, and was an array here (arrayLiteralOperands).
		if inner, ok := parenthesisedList(value); ok && (options.indexed || options.associative || r.arrayOperands[argument]) {
			if status := r.assignCompound(ctx, name, inner, appended, 0); status != 0 {
				return status
			}
		} else {
			if appended {
				value = r.appendedValue(name, value)
			}
			if status := r.assignVar(name, value); status != 0 {
				return status
			}
		}
	}
	if options.export {
		r.markExported(name)
	}
	if strings.ContainsRune(options.removed, 'x') {
		r.unexport(name)
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
		key := r.resolveKey(ctx, reference.subscript)
		// An empty key names nothing, and bash refuses it and ends the script, as an
		// assignment error does; here it was a key of its own.
		if key == "" {
			fmt.Fprintf(r.streams.Stderr, "%s[%s]: bad array subscript\n", reference.name, reference.subscript)
			r.raiseShellError()
			return 1
		}
		r.arrays.setKey(reference.name, key, value)
		return 0
	}
	return r.assignArrayElementText(ctx, reference, value)
}

// printDeclarations is `declare -p name...`; with no names it is listDeclarations.
//
// The form is bash's, because it is meant to be read back by the shell: a `declare -p`
// whose output cannot be pasted into a script is a listing, not a declaration.
//
// A name that is not there is 1, as in bash, and the others are printed all the same.
func (r Runtime) printDeclarations(names []string) int {
	status := 0
	for _, name := range names {
		if !r.printOneDeclaration(strings.SplitN(name, "=", 2)[0]) {
			status = 1
		}
	}
	return status
}

func (r Runtime) printOneDeclaration(name string) bool {
	text, found := r.declarationText(name)
	if !found {
		fmt.Fprintf(r.streams.Stderr, "declare: %s: not found\n", name)
		return false
	}
	fmt.Fprintln(r.streams.Stdout, text)
	return true
}

// declarationText is a name written as the declaration that recreates it, which is what
// `declare -p` prints and what `${a[@]@A}` expands to for an array.
//
// With the name's attributes as flags, in bash's order: `declare -irx n="5"`. It printed
// `-a`, `-A` or `--` whatever else was true of the name.
//
// Each value and key is quoted as bash quotes it, see internal/shellquote. This used Go's
// quoting, which is not the shell's: `x=$'a\nb'` came back as the two characters \n, a `$`
// was left to expand when the line was read back, and a key with a blank came out bare.
func (r Runtime) declarationText(name string) (string, bool) {
	flags := r.declareFlags(name)
	if list, ok := r.arrayLiteralText(name); ok {
		return fmt.Sprintf("declare -%s %s=%s", flags, name, list), true
	}
	value, set := r.vars[name]
	if !set {
		if flags != "-" {
			// Declared and never assigned: bash prints the attributes and no value.
			return fmt.Sprintf("declare -%s %s", flags, name), true
		}
		return "", false
	}
	return fmt.Sprintf("declare -%s %s=%s", flags, name, shellquote.Double(value)), true
}

// arrayLiteralText is an array as the compound value that recreates it, `([0]="a" [2]="c")`
// or `([k]="v" )`: what declare -p writes after the name, and a bare `set` after the `=`.
func (r Runtime) arrayLiteralText(name string) (string, bool) {
	if r.arrays == nil {
		return "", false
	}
	var out strings.Builder
	out.WriteString("(")
	switch {
	case r.arrays.isAssociative(name):
		for _, key := range r.arrays.keysOf(name) {
			value, _ := r.arrays.lookupKey(name, key)
			fmt.Fprintf(&out, "[%s]=%s ", shellquote.Key(key), shellquote.Double(value))
		}
		// bash leaves the blank before the closing parenthesis for this kind and not
		// the other, and output meant to be read back is worth matching exactly.
		return out.String() + ")", true
	case r.arrays.has(name):
		// The set indices only: every slot used to be printed, so a gap came out as
		// `[1]=""` and an element removed with unset came back with its old value.
		for _, index := range r.arrays.liveIndices(name) {
			value, _ := r.arrays.valueAt(name, index)
			fmt.Fprintf(&out, "[%d]=%s ", index, shellquote.Double(value))
		}
		return strings.TrimSuffix(out.String(), " ") + ")", true
	}
	return "", false
}

// parenthesisedList reports the inside of a `(one two)` array literal.
func parenthesisedList(value string) (string, bool) {
	if !strings.HasPrefix(value, "(") || !strings.HasSuffix(value, ")") {
		return "", false
	}
	return value[1 : len(value)-1], true
}
