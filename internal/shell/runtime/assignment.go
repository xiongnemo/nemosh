package runtime

import (
	"context"
	"maps"
	"strings"
)

type assignment struct {
	name  string
	value string
	// appended is `name+=value`; see variable_attributes.go.
	appended bool
}

func leadingAssignments(args []string) ([]assignment, []string) {
	assignments := make([]assignment, 0, len(args))
	for i, arg := range args {
		if !isAssignment(arg) {
			return assignments, args[i:]
		}
		target, value, _ := cutAssignment(arg)
		name, appended := splitAssignmentTarget(target)
		assignments = append(assignments, assignment{name: name, value: value, appended: appended})
	}
	return assignments, nil
}

// splitAssignments separates a command's leading assignments from the command itself, at
// the count of assignment words it began with. Decided on the words as written, not on what
// they expanded to: a word that expands to `name=value` is a command name, as are a quoted
// one and one with its `=` escaped -- `v='a=b'; $v`, `"c=d"` and `e\=f` each run a command of
// that name in busybox-w32 and bash, where every one of them was taken for an assignment.
func splitAssignments(args []string, count int) ([]assignment, []string) {
	counted := make([]string, count)
	for index, arg := range args[:count] {
		counted[index] = keepEmptySubscript(arg)
	}
	assignments, rest := leadingAssignments(counted)
	return assignments, append(rest, args[count:]...)
}

// keepEmptySubscript keeps an element assignment whose subscript came to nothing an element
// assignment. `a[$e]=1` with e empty, or `m[""]=1`, is `a[]=1` once expanded, which is not an
// assignment's shape, and was run as a command of that name; bash assigns element 0. The
// subscript is written `""` instead, which an indexed array reads as 0 and an associative
// one refuses, as bash has them.
func keepEmptySubscript(arg string) string {
	open := strings.Index(arg, "[]")
	if open <= 0 || !isValidVariableName(arg[:open]) {
		return arg
	}
	if rest := arg[open+2:]; strings.HasPrefix(rest, "=") || strings.HasPrefix(rest, "+=") {
		return arg[:open] + `[""]` + rest
	}
	return arg
}

// assignedValue is what an assignment stores: its value, or the old one with it for `+=`.
func (r Runtime) assignedValue(assignment assignment) string {
	if assignment.appended {
		return r.appendedValue(assignment.name, assignment.value)
	}
	return assignment.value
}

func (r Runtime) assignVars(assignments []assignment) int {
	plain := r.assigningPlainly()
	for _, assignment := range assignments {
		if status := plain.assignVar(assignment.name, plain.assignedValue(assignment)); status != 0 {
			return status
		}
	}
	return 0
}

// assigningPlainly marks the writes of an assignment statement, which end the script when
// they cannot be made; see failAssignment.
func (r Runtime) assigningPlainly() Runtime {
	r.plainAssignment = true
	return r
}

// failAssignment abandons the command an assignment statement's failed write was in, status
// 1, as bash does: a subscript before the front of an array, a list for one element, an
// integer's value that is no expression. As with failglob's, the whole command the shell was
// running goes -- the function it called, the loop it was in -- and the script goes on with
// the next, unless `set -e` ends it there; see flowDiscard. Each was said and the rest of the
// command went on as if nothing had happened. The same write from declare, read or printf -v
// is status 1 and the next command there, and so is a nameref's circle anywhere; a read-only
// name has raised its own, busybox's status 2 and the end of the script. An element of a
// compound assignment abandons the command wherever it was written; see refuseElement.
func (r Runtime) failAssignment() {
	if r.plainAssignment && !r.expansion.shellError {
		r.expansion.shellError, r.expansion.discard = true, true
	}
}

func (r Runtime) runCommandWithLeadingAssignments(ctx context.Context, args []string) int {
	assignments, commandArgs := leadingAssignments(args)
	if len(commandArgs) == 0 {
		return r.assignVars(assignments)
	}
	if len(assignments) == 0 {
		return r.runCommandWithRedirects(ctx, commandArgs)
	}
	if isSpecialBuiltin(commandArgs[0]) {
		if status := r.assignVars(assignments); status != 0 {
			return status
		}
		return r.runCommandWithRedirects(ctx, commandArgs)
	}
	return r.runCommandWithTemporaryAssignments(ctx, commandArgs, assignments)
}

func (r Runtime) runCommandWithTemporaryAssignments(ctx context.Context, args []string, assignments []assignment) int {
	commandRuntime := r.withLocalAssignments(assignments)
	if commandRuntime == nil {
		return 1
	}
	status := commandRuntime.runCommandWithRedirects(ctx, args)
	r.mergeBuiltinMutations(*commandRuntime)
	return status
}

func (r Runtime) withLocalAssignments(assignments []assignment) *Runtime {
	commandRuntime := r
	commandRuntime.vars = make(map[string]string, len(r.vars)+len(assignments))
	commandRuntime.env = r.env.clone()
	commandRuntime.shadowed = nil
	maps.Copy(commandRuntime.vars, r.vars)
	for _, assignment := range assignments {
		// See array_shadow.go: an array's name is a string for the command.
		commandRuntime.shadowArray(assignment.name)
		if status := commandRuntime.assignVar(assignment.name, commandRuntime.assignedValue(assignment)); status != 0 {
			r.restoreShadowed(commandRuntime)
			return nil
		}
		// What was stored, which `+=` and an attribute can make differ from what was
		// written: the command's environment sees the same value its shell does.
		commandRuntime.env.Set(assignment.name, commandRuntime.vars[assignment.name])
	}
	commandRuntime.mutatedVars = make(map[string]struct{})
	return &commandRuntime
}

func (r Runtime) mergeBuiltinMutations(commandRuntime Runtime) {
	r.restoreShadowed(commandRuntime)
	for name := range commandRuntime.mutatedVars {
		value, exists := commandRuntime.vars[name]
		if exists {
			r.vars[name] = value
		} else {
			delete(r.vars, name)
		}
		if value, exported := commandRuntime.env.LookupEnv(name); exported {
			r.env.Set(name, value)
		} else {
			r.env.Unset(name)
		}
		if commandRuntime.isReadonly(name) {
			r.readonly[name] = struct{}{}
		}
	}
}
