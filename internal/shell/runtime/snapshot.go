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
		initErr:       r.initErr,
		registry:      r.registry,
		functions:     cloneMap(r.functions),
		streams:       table.streams(),
		fds:           table,
		vars:          cloneMap(r.vars),
		traps:         cloneMap(r.traps),
		trapRunning:   map[trapName]bool{},
		params:        &parameters{name: r.params.name, values: append([]string(nil), r.params.values...)},
		options:       &shellOptions{pipefail: r.options.pipefail},
		readonly:      cloneMap(r.readonly),
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
