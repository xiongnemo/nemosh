package runtime

import (
	"context"
	"errors"
	"maps"
)

func (r Runtime) snapshot(ctx context.Context) (Runtime, error) {
	return r.clone(ctx, true)
}

func (r Runtime) snapshotShared() (Runtime, error) {
	return r.clone(context.Background(), false)
}

// runOwnExitTrap is a subshell's end: its EXIT trap runs if the subshell set one itself,
// and not if the one it has is only the parent's, which fires once, when the parent exits.
// Both references agree: `(trap 'echo bye' EXIT; ...)` says bye as the subshell ends, which
// here it never did. The parent's traps stay visible to the subshell all the same, so the
// save-and-restore idiom `saved=$(trap)` sees them -- a command substitution started with an
// empty table, and that idiom saved nothing.
func (r Runtime) runOwnExitTrap(ctx context.Context, inherited string, status int) {
	if own := r.traps[trapExit]; own != "" && own != inherited {
		r.runExitTrap(context.WithoutCancel(ctx), status)
	}
}

// inheritedTraps are the traps a snapshot starts with: all of them, except ERR unless
// `set -E` asks for it. In both references `(false)` fires the trap once, in the parent,
// for the subshell's status -- not a second time inside it.
func (r Runtime) inheritedTraps() map[trapName]string {
	traps := cloneMap(r.traps)
	if !r.options.errTrace {
		delete(traps, trapERR)
	}
	return traps
}

func (r Runtime) clone(ctx context.Context, privateJobs bool) (Runtime, error) {
	paths := *r.paths
	table, err := r.fds.clone()
	if err != nil {
		return Runtime{}, err
	}
	jobs := r.jobScope
	lifecycle := &shellLifecycle{}
	if privateJobs {
		jobs = newPrivateJobScope(ctx, r.jobScope.supervisor)
	} else {
		lifecycle = r.lifecycle
	}
	return Runtime{
		initErr:     r.initErr,
		registry:    r.registry,
		functions:   cloneMap(r.functions),
		streams:     table.streams(),
		fds:         table,
		vars:        cloneMap(r.vars),
		traps:       r.inheritedTraps(),
		trapRunning: map[trapName]bool{},
		params:      &parameters{name: r.params.name, values: append([]string(nil), r.params.values...), function: r.params.function},
		options:     r.options.clone(),
		expansion:   newExpansionState(),
		aliases:     cloneMap(r.aliases),
		childCPU:    r.childCPU,
		history:     r.history,
		// A subshell starts with no pending break: `(break)` inside a loop does
		// not break the loop outside it, because the loop is not in the subshell.
		loops: newLoopLevels(),
		// Shared: $SECONDS in a subshell counts from when the shell started, not
		// from when the subshell did.
		special: r.special,
		// Cloned rather than shared, and never omitted: see shellArrays.clone.
		arrays: r.arrays.clone(),
		// Cloned for the same reason arrays are: `(pushd /tmp)` must leave the
		// parent's stack alone. $SECONDS and history are the deliberate exceptions
		// above; a directory stack is not one of them.
		dirStack: r.dirStack.clone(),
		// locals belongs to a function call, and a snapshot is not inside
		// one: a subshell or a background worker that returns has nothing
		// of the caller's to restore.
		locals:        nil,
		frames:        r.frames,
		scriptFile:    r.scriptFile,
		readonly:      cloneMap(r.readonly),
		attributes:    cloneMap(r.attributes),
		mutatedVars:   cloneMap(r.mutatedVars),
		mask:          &fileModeMask{value: r.mask.value},
		sourceDepth:   r.sourceDepth,
		functionDepth: r.functionDepth,
		interactive:   r.interactive,
		paths:         &paths,
		env:           r.env.clone(),
		jobScope:      jobs,
		lifecycle:     lifecycle,
	}, nil
}

func (r Runtime) withFDTable(table *fdTable) Runtime {
	r.fds = table
	r.streams = table.streams()
	return r
}

func (r Runtime) withStreams(streams Streams) (Runtime, error) {
	if err := r.fds.bindBorrowedReader(0, streams.Stdin); err != nil {
		return r, errors.Join(err, r.fds.closeAll())
	}
	if err := r.fds.bindBorrowedWriter(1, streams.Stdout); err != nil {
		return r, errors.Join(err, r.fds.closeAll())
	}
	if err := r.fds.bindBorrowedWriter(2, streams.Stderr); err != nil {
		return r, errors.Join(err, r.fds.closeAll())
	}
	return r.withFDTable(r.fds), nil
}

func cloneMap[K comparable, V any](source map[K]V) map[K]V {
	cloned := make(map[K]V, len(source))
	maps.Copy(cloned, source)
	return cloned
}
