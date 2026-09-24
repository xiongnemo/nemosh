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
		target, value, _ := strings.Cut(arg, "=")
		name, appended := splitAssignmentTarget(target)
		assignments = append(assignments, assignment{name: name, value: value, appended: appended})
	}
	return assignments, nil
}

// assignedValue is what an assignment stores: its value, or the old one with it for `+=`.
func (r Runtime) assignedValue(assignment assignment) string {
	if assignment.appended {
		return r.appendedValue(assignment.name, assignment.value)
	}
	return assignment.value
}

func (r Runtime) assignVars(assignments []assignment) int {
	for _, assignment := range assignments {
		if status := r.assignVar(assignment.name, r.assignedValue(assignment)); status != 0 {
			return status
		}
	}
	return 0
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
	maps.Copy(commandRuntime.vars, r.vars)
	for _, assignment := range assignments {
		if status := commandRuntime.assignVar(assignment.name, commandRuntime.assignedValue(assignment)); status != 0 {
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
