package runtime

import (
	"context"
	"maps"
	"strings"
)

type assignment struct {
	name  string
	value string
}

func leadingAssignments(args []string) ([]assignment, []string) {
	assignments := make([]assignment, 0, len(args))
	for i, arg := range args {
		if !isAssignment(arg) {
			return assignments, args[i:]
		}
		name, value, _ := strings.Cut(arg, "=")
		assignments = append(assignments, assignment{name: name, value: value})
	}
	return assignments, nil
}

func (r Runtime) assignVars(assignments []assignment) int {
	for _, assignment := range assignments {
		if status := r.assignVar(assignment.name, assignment.value); status != 0 {
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
		if status := commandRuntime.assignVar(assignment.name, assignment.value); status != 0 {
			return nil
		}
		commandRuntime.env.Set(assignment.name, assignment.value)
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
