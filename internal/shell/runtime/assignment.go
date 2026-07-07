package runtime

import (
	"context"
	"fmt"
	"os"
	"strings"
)

type assignment struct {
	name  string
	value string
}

type envRestore struct {
	name    string
	value   string
	present bool
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
	return r.runCommandWithTemporaryAssignments(ctx, commandArgs, assignments)
}

func (r Runtime) runCommandWithTemporaryAssignments(ctx context.Context, args []string, assignments []assignment) int {
	commandRuntime := r.withLocalAssignments(assignments)
	if commandRuntime == nil {
		return 1
	}
	restore, status := r.applyTemporaryEnvironment(assignments)
	if status != 0 {
		return status
	}
	commandStatus := commandRuntime.runCommandWithRedirects(ctx, args)
	if err := restore(); err != nil && commandStatus == 0 {
		fmt.Fprintf(r.streams.Stderr, "assignment: %v\n", err)
		return 1
	}
	return commandStatus
}

func (r Runtime) withLocalAssignments(assignments []assignment) *Runtime {
	commandRuntime := r
	commandRuntime.vars = make(map[string]string, len(r.vars)+len(assignments))
	for name, value := range r.vars {
		commandRuntime.vars[name] = value
	}
	for _, assignment := range assignments {
		if status := commandRuntime.assignVar(assignment.name, assignment.value); status != 0 {
			return nil
		}
	}
	return &commandRuntime
}

func (r Runtime) applyTemporaryEnvironment(assignments []assignment) (func() error, int) {
	restores := make([]envRestore, 0, len(assignments))
	for _, assignment := range assignments {
		value, present := os.LookupEnv(assignment.name)
		restores = append(restores, envRestore{name: assignment.name, value: value, present: present})
		if err := os.Setenv(assignment.name, assignment.value); err != nil {
			if restoreErr := restoreEnvironment(restores); restoreErr != nil {
				fmt.Fprintf(r.streams.Stderr, "assignment: %v\n", restoreErr)
			}
			fmt.Fprintf(r.streams.Stderr, "assignment: %s: %v\n", assignment.name, err)
			return func() error { return nil }, 1
		}
	}
	return func() error { return restoreEnvironment(restores) }, 0
}

func restoreEnvironment(restores []envRestore) error {
	for i := len(restores) - 1; i >= 0; i-- {
		restore := restores[i]
		if restore.present {
			if err := os.Setenv(restore.name, restore.value); err != nil {
				return err
			}
			continue
		}
		if err := os.Unsetenv(restore.name); err != nil {
			return err
		}
	}
	return nil
}
