package runtime

import (
	"context"
	"fmt"
)

// `>(command)` -- the output half of process substitution.
//
// `tee >(sha256sum > sum) > copy` and `exec > >(tee -a log) 2>&1` are the shapes it exists
// for: a command's output fed to another while it runs. It was refused by name, because the
// temporary file `<(cmd)` uses cannot do it: the reader would start only once the writer had
// finished, and `exec > >(tee log)` never finishes.
//
// So this one is a pipe the consumer opens by path -- a named pipe on Windows, a FIFO
// elsewhere (substitution_pipe_*.go) -- and the command runs concurrently, reading it, as in
// bash. Two differences remain, both stated in the support matrix. bash leaves the command
// running if the shell exits first; here it is a goroutine of the shell, so a script waits
// for it before exiting, and the output is the same. And a path the consumer never opens is
// opened and closed again after the command, so the substituted command reads an empty
// input and ends, which is what bash's does when its pipe's writer goes away.

// expandOutputSubstitution starts the command reading from a new pipe and answers with the
// pipe's path.
func (r Runtime) expandOutputSubstitution(ctx context.Context, script *Script, savedStatus int) string {
	if script == nil {
		return ""
	}
	// Not ended with the command that expanded it: the consumer finishing is not the
	// substituted command finishing, and `exec > >(tee log)` outlives every command after
	// it. It ends when its input does.
	lifetime := context.WithoutCancel(ctx)
	child, err := r.snapshot(lifetime)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: process substitution: %v\n", err)
		return ""
	}
	pipe, err := newSubstitutionPipe()
	if err != nil {
		child.jobScope.cancelAndDrain()
		_ = child.fds.closeAll()
		fmt.Fprintf(r.streams.Stderr, "nemosh: process substitution: %v\n", err)
		return ""
	}
	r.expansion.outputPipes = append(r.expansion.outputPipes, pipe)
	if r.substitutions != nil {
		r.substitutions.Add(1)
	}
	go func() {
		if r.substitutions != nil {
			defer r.substitutions.Done()
		}
		defer pipe.remove()
		child.guardedRun("running a process substitution", func() lineResult {
			child.readSubstitution(lifetime, *script, pipe, savedStatus)
			return lineResult{}
		})
	}()
	return pipe.path
}

// readSubstitution runs the script with the pipe as its input, once the consumer has
// opened it. The child is a snapshot, as runIntoFile's is, with no inherited traps.
func (child Runtime) readSubstitution(ctx context.Context, script Script, pipe *substitutionPipe, savedStatus int) {
	table := child.fds
	defer func() {
		child.jobScope.cancelAndDrain()
		_ = table.closeAll()
	}()
	reader, err := pipe.accept()
	if err != nil {
		fmt.Fprintf(child.streams.Stderr, "nemosh: process substitution: %v\n", err)
		return
	}
	if err := table.bindOwnedReader(0, reader); err != nil {
		_ = reader.Close()
		fmt.Fprintf(child.streams.Stderr, "nemosh: process substitution: %v\n", err)
		return
	}
	child = child.withFDTable(table)
	child.traps = map[trapName]string{}
	child.executeTypedScriptFrom(ctx, script, savedStatus)
}

// abandonOutputPipes lets each pipe's command go on once the consumer is done with the
// path, whether it opened it or not; see substitutionPipe.abandon.
func (r Runtime) abandonOutputPipes() {
	pipes := r.expansion.outputPipes
	r.expansion.outputPipes = nil
	for _, pipe := range pipes {
		pipe.abandon()
	}
}
